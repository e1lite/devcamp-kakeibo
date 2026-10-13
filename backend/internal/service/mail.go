package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/auth"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/extract"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/gmail"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/mailparse"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/model"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/repository"
)

// メール連携・同期のエラー。ハンドラがこれを見てエラーコードを出し分ける（API-006〜009 / 029 / 030）。
var (
	// ErrInsufficientScope は同意画面で gmail.readonly のチェックを外された。
	ErrInsufficientScope = errors.New("メールの読み取りが許可されていません")
	// ErrSyncAlreadyRunning は同じ連携の同期が実行中。409 に対応する。
	ErrSyncAlreadyRunning = errors.New("同期が実行中です")
	// ErrAccountDisabled は連携が解除済み。409 に対応する。
	ErrAccountDisabled = errors.New("連携が解除されています")
	// ErrMailReauthRequired は認証情報が失効していて、連携し直す必要がある。409 に対応する。
	ErrMailReauthRequired = errors.New("連携し直す必要があります")
	// ErrAlreadyDisabled はすでに解除済みの連携を解除しようとした。409 に対応する。
	ErrAlreadyDisabled = errors.New("すでに解除されています")
)

const (
	// 1 回の同期で取り込む上限。初回の遡り取得で、想定外に多くのメールが条件に合ったときの歯止め
	maxMessagesPerSync = 500
	// 1 回の同期にかける時間の上限
	syncTimeout = 10 * time.Minute
	// 差分取得で前回の同期より前に遡る幅。受信が遅れたメールを取りこぼさないため、期間を重ねて取る。
	// 重ねて取った分は UNIQUE(mail_account_id, provider_message_id) で 1 行にしかならない
	// （docs/mail-import-mvp.md 判断 5）
	syncOverlap = 24 * time.Hour
)

// MailConfig は Mail サービスの依存と設定。
type MailConfig struct {
	// OAuth はメール連携用の設定。スコープは gmail.readonly、戻り先は API-030
	OAuth               *oauth2.Config
	States              *auth.StateStore
	Cipher              *auth.CredentialCipher
	Accounts            *repository.MailAccount
	Jobs                *repository.SyncJob
	Messages            *repository.EmailMessage
	Templates           *repository.ParserTemplate
	AllowedRedirectURIs []string
	DefaultRedirectURI  string
	SearchQuery         string
	BackfillDays        int
	GmailBaseURL        string
}

// Mail はメール連携と、メールの取込（パイプライン P0〜P4）を担う。
//
// 取得したメールを email_messages に保存し（P1）、テンプレートで判定・抽出して（P2・P3）、
// 決済イベントと取引を作る。最小版は名寄せ（P5）と自動分類（P6）をしない（docs/mail-import-mvp.md）。
type Mail struct {
	cfg MailConfig
	now func() time.Time
	// 同じ連携の同期が同時に 2 つ始まらないよう、確認と作成をまとめて行う
	startMu sync.Mutex
}

// NewMail は Mail サービスを生成する。
func NewMail(cfg MailConfig) *Mail {
	return &Mail{cfg: cfg, now: time.Now}
}

// --- 連携（API-029 / 030） ------------------------------------------------

// AuthorizeURL は Gmail の読み取りの同意画面の URL を作る（API-029）。
func (s *Mail) AuthorizeURL(ctx context.Context, userID int64, redirectURI string, now time.Time) (string, error) {
	if redirectURI == "" {
		redirectURI = s.cfg.DefaultRedirectURI
	}
	if !s.IsAllowedRedirectURI(redirectURI) {
		return "", ErrInvalidRedirectURI
	}

	state, err := s.cfg.States.Issue(auth.StateData{
		Purpose:     auth.PurposeMailLink,
		RedirectURI: redirectURI,
		UserID:      userID,
	}, now)
	if err != nil {
		return "", err
	}

	// ユーザがいなくても定期的にメールを読むには、リフレッシュトークンが要る（access_type=offline）
	options := []oauth2.AuthCodeOption{oauth2.AccessTypeOffline}

	// アカウントの選択画面は毎回出す（select_account）。連携するのはログインと別の Gmail のことが多く、
	// ブラウザがログイン中の別のアカウント（勤務先など）で、黙って連携してしまわないようにする。
	//
	// consent は、使える連携がまだ無いときだけ付ける（Step 4 のフィードバック、判断 7）。
	// Google はリフレッシュトークンを最初の同意のときにしか返さないため、新規・再認証では
	// 同意画面を出し直して受け取る。使える連携があるのに毎回出すと、利用者の手間が増える
	prompt := "select_account"
	active, err := s.cfg.Accounts.CountActive(ctx, userID)
	if err != nil {
		return "", err
	}
	if active == 0 {
		prompt += " consent"
	}
	options = append(options, oauth2.SetAuthURLParam("prompt", prompt))

	return s.cfg.OAuth.AuthCodeURL(state, options...), nil
}

// IsAllowedRedirectURI は戻り先が許可リストにあるかを返す。
func (s *Mail) IsAllowedRedirectURI(uri string) bool {
	return slices.Contains(s.cfg.AllowedRedirectURIs, uri)
}

// DefaultRedirectURI は state を検証できなかったときの戻り先（メール連携画面）。
func (s *Mail) DefaultRedirectURI() string {
	return s.cfg.DefaultRedirectURI
}

// ConsumeState はメール連携の state を検証する（API-030）。
// ログイン用の state はここでは使えない。
func (s *Mail) ConsumeState(state string, now time.Time) (auth.StateData, error) {
	data, err := s.cfg.States.Consume(state, auth.PurposeMailLink, now)
	if err != nil {
		return auth.StateData{}, ErrInvalidState
	}
	return data, nil
}

// LinkResult は連携の完了後にハンドラへ返す情報。
type LinkResult struct {
	Account *model.MailAccount
	IsNew   bool
}

// CompleteLink は認可コードを認証情報に交換して、連携を保存する（API-030）。
// 新規の連携なら、過去メールの取込（backfill）を始める。
func (s *Mail) CompleteLink(ctx context.Context, userID int64, code string) (*LinkResult, error) {
	token, err := s.cfg.OAuth.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrOAuthExchange, err)
	}

	// 同意画面ではスコープごとに拒否できる。gmail.readonly が外されていたら連携しない
	scope, _ := token.Extra("scope").(string)
	if !slices.Contains(strings.Fields(scope), gmail.ReadonlyScope) {
		return nil, ErrInsufficientScope
	}

	// 連携したアドレスは、ログインしたアドレスと違ってよい（1 ユーザが複数のアドレスを連携できる）
	client := gmail.NewClient(s.cfg.OAuth.Client(ctx, token), s.cfg.GmailBaseURL)
	profile, err := client.Profile(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: 連携したアドレスを取得できません: %w", ErrOAuthExchange, err)
	}
	email := strings.ToLower(profile.EmailAddress)

	existing, err := s.cfg.Accounts.FindByUserAndEmail(ctx, userID, email)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}

	credential := ""
	if token.RefreshToken != "" {
		credential, err = s.cfg.Cipher.Encrypt(token.RefreshToken)
		if err != nil {
			return nil, err
		}
	} else if existing == nil || existing.CredentialEncrypted == "" {
		// 同意画面を出さずに戻ってきたため、リフレッシュトークンを受け取れなかった。
		// 使える認証情報が手元に無いので、連携として保存しない
		return nil, fmt.Errorf("%w: リフレッシュトークンを受け取れませんでした", ErrOAuthExchange)
	}

	if existing != nil {
		// 再連携（reauth_required / disabled からの復帰を含む）。認証情報を差し替えて使える状態に戻す
		fields := map[string]any{"status": model.MailAccountStatusActive}
		if credential != "" {
			fields["credential_encrypted"] = credential
		}
		if err := s.cfg.Accounts.Update(ctx, existing.ID, fields); err != nil {
			return nil, err
		}
		account, err := s.cfg.Accounts.FindByID(ctx, existing.ID)
		if err != nil {
			return nil, err
		}
		return &LinkResult{Account: account}, nil
	}

	account := &model.MailAccount{
		UserID:              userID,
		EmailAddress:        email,
		Provider:            model.MailAccountProviderGmailAPI,
		CredentialEncrypted: credential,
		Status:              model.MailAccountStatusActive,
	}
	if err := s.cfg.Accounts.Create(ctx, account); err != nil {
		return nil, err
	}

	// 新規なら、連携した直後に過去のメールを取り込む（API-030 の 5）
	if _, err := s.startSync(ctx, account, model.SyncTriggerBackfill, true); err != nil {
		slog.Error("連携直後の取込を始められませんでした", "mail_account_id", account.ID, "error", err)
	}
	return &LinkResult{Account: account, IsNew: true}, nil
}

// --- 連携の一覧・解除（API-006 / 007） -------------------------------------

// ListAccounts はユーザの連携を返す（API-006）。
func (s *Mail) ListAccounts(ctx context.Context, userID int64) ([]model.MailAccount, error) {
	return s.cfg.Accounts.ListByUser(ctx, userID)
}

// Disconnect は連携を解除する（API-007）。
//
// 行は削除しない。取り込んだメールと家計簿は残し、「これ以上メールを読まない」ために
// 認証情報だけを破棄する（DB 仕様書 3.2 の削除方針）。
func (s *Mail) Disconnect(ctx context.Context, userID, accountID int64) error {
	account, err := s.findOwned(ctx, userID, accountID)
	if err != nil {
		return err
	}
	if account.Status == model.MailAccountStatusDisabled {
		return ErrAlreadyDisabled
	}
	return s.cfg.Accounts.Update(ctx, account.ID, map[string]any{
		"status":               model.MailAccountStatusDisabled,
		"credential_encrypted": "",
	})
}

// findOwned は連携を取得し、所有者であることを確かめる。
// 他ユーザの連携は存在しない場合と同じ ErrNotFound にする（API 仕様書 6 章冒頭）。
func (s *Mail) findOwned(ctx context.Context, userID, accountID int64) (*model.MailAccount, error) {
	account, err := s.cfg.Accounts.FindByID(ctx, accountID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if account.UserID != userID {
		return nil, ErrNotFound
	}
	return account, nil
}

// --- 同期（API-008 / 009） ------------------------------------------------

// RequestSync は手動の同期を始める（API-008）。backfill なら、前回の同期によらず過去のメールを遡る。
//
// 取得には数十秒かかりうるため、ジョブを作ったら完了を待たずに返す（202）。
// 進み具合は API-009 で確かめる。
func (s *Mail) RequestSync(ctx context.Context, userID, accountID int64, backfill bool) (*model.SyncJob, error) {
	account, err := s.findOwned(ctx, userID, accountID)
	if err != nil {
		return nil, err
	}
	switch account.Status {
	case model.MailAccountStatusDisabled:
		return nil, ErrAccountDisabled
	case model.MailAccountStatusReauthRequired:
		return nil, ErrMailReauthRequired
	}
	return s.startSync(ctx, account, model.SyncTriggerManual, backfill)
}

func (s *Mail) startSync(ctx context.Context, account *model.MailAccount, trigger string, backfill bool) (*model.SyncJob, error) {
	s.startMu.Lock()
	defer s.startMu.Unlock()

	running, err := s.cfg.Jobs.HasRunning(ctx, account.ID)
	if err != nil {
		return nil, err
	}
	if running {
		return nil, ErrSyncAlreadyRunning
	}

	job := &model.SyncJob{
		MailAccountID: account.ID,
		Trigger:       trigger,
		StartedAt:     s.now(),
		Status:        model.SyncStatusRunning,
	}
	if err := s.cfg.Jobs.Create(ctx, job); err != nil {
		return nil, err
	}

	// ジョブは API サーバの中で動かす。別のワーカーは作らない（docs/mail-import-mvp.md 判断 4）。
	// リクエストが終わっても続けるため、リクエストの取り消しは引き継がない
	go s.runSync(context.WithoutCancel(ctx), *job, *account, backfill)
	return job, nil
}

// ListJobs はユーザの同期履歴を返す（API-009）。
func (s *Mail) ListJobs(ctx context.Context, userID int64, filter repository.SyncJobFilter) ([]model.SyncJob, int64, error) {
	if filter.Limit > MaxLimit {
		return nil, 0, ErrLimitTooLarge
	}
	if filter.Limit <= 0 {
		filter.Limit = DefaultLimit
	}
	return s.cfg.Jobs.List(ctx, userID, filter)
}

// RecoverInterrupted は、前回サーバを止めたときに実行中のまま残ったジョブを失敗にする。起動時に呼ぶ。
func (s *Mail) RecoverInterrupted(ctx context.Context) error {
	count, err := s.cfg.Jobs.FailRunning(ctx, s.now(), "サーバの停止で同期が中断されました")
	if err != nil {
		return err
	}
	if count > 0 {
		slog.Warn("中断された同期ジョブを失敗にしました", "count", count)
	}
	return nil
}

// syncOutcome は 1 回の同期の結果。
type syncOutcome struct {
	fetched int
	// parsed は取引を作れたメールの数、failed は取得・抽出に失敗したメールの数
	parsed    int
	failed    int
	truncated bool
	since     time.Time
}

// runSync は同期を実行し、結果をジョブと連携に記録する。
//
// 取得（P1）のあと、未処理のメールをすべて判定・抽出する（P2〜P4）。
// 前回までに取得して未処理のまま残ったメール（テンプレートを足す前に取得したものなど）もここで処理する。
func (s *Mail) runSync(parent context.Context, job model.SyncJob, account model.MailAccount, backfill bool) {
	ctx, cancel := context.WithTimeout(parent, syncTimeout)
	defer cancel()

	logger := slog.With("sync_job_id", job.ID, "mail_account_id", account.ID)

	var outcome syncOutcome
	// まだ一度も同期に成功していなければ、手動の同期でも過去のメールを遡る（初回の取込が途中で失敗した場合など）
	fullRange := backfill || account.LastSyncedAt == nil

	templates, err := s.loadTemplates(ctx, account.UserID)
	if err == nil {
		outcome, err = s.fetchMessages(ctx, &account, searchQuery(templates, s.cfg.SearchQuery), fullRange)

		// 判定・抽出は保存済みのメールだけで行え、Gmail に繋がらなくてもできる。
		// 取得が途中で失敗しても（認証の失効・利用上限など）、取得できた分は取引にする
		parsed, failed, perr := s.processPending(ctx, &account, templates)
		outcome.parsed = parsed
		outcome.failed += failed
		if err == nil {
			err = perr
		} else if perr != nil {
			logger.Error("保存済みのメールの判定・抽出に失敗しました", "error", perr)
		}
	}
	finished := s.now()

	// 結果の記録は、取得の時間切れ（ctx の期限切れ）でも書けるよう、別の context で行う。
	// 書けないとジョブが実行中のまま残り、次の同期を始められなくなる
	ctx, cancelRecord := context.WithTimeout(parent, 30*time.Second)
	defer cancelRecord()

	fields := map[string]any{
		"finished_at":   finished,
		"fetched_count": outcome.fetched,
		"parsed_count":  outcome.parsed,
		"failed_count":  outcome.failed,
	}

	if err != nil {
		logger.Error("同期に失敗しました", "error", err)
		fields["status"] = model.SyncStatusFailed
		fields["error_message"] = err.Error()
		if errors.Is(err, gmail.ErrReauthRequired) || errors.Is(err, auth.ErrCredentialDecrypt) {
			// 連携し直すまで同期できないことを、連携の一覧（API-006）とヘッダー（API-004 の needs_reauth）で示す
			if uerr := s.cfg.Accounts.Update(ctx, account.ID, map[string]any{
				"status": model.MailAccountStatusReauthRequired,
			}); uerr != nil {
				logger.Error("連携の状態を更新できませんでした", "error", uerr)
			}
		}
	} else {
		fields["status"] = model.SyncStatusSuccess
		if outcome.failed > 0 || outcome.truncated {
			// 一部のメールを取り込めなかった。取り込めた分は残す
			fields["status"] = model.SyncStatusPartial
		}
		if outcome.truncated {
			fields["error_message"] = fmt.Sprintf("1 回の上限（%d 件）に達したため、残りは次の同期で取り込みます", maxMessagesPerSync)
		}

		// 0 件でも、成功したら最終同期日時を更新する（死活監視に使う。DB 仕様書 3.3）
		accountFields := map[string]any{"last_synced_at": finished}
		if fullRange && !outcome.truncated {
			accountFields["backfilled_until"] = outcome.since
		}
		if uerr := s.cfg.Accounts.Update(ctx, account.ID, accountFields); uerr != nil {
			logger.Error("最終同期日時を更新できませんでした", "error", uerr)
		}
		logger.Info("同期しました", "fetched", outcome.fetched, "parsed", outcome.parsed, "failed", outcome.failed)
	}

	if uerr := s.cfg.Jobs.Update(ctx, job.ID, fields); uerr != nil {
		logger.Error("同期ジョブの結果を記録できませんでした", "error", uerr)
	}
}

// fetchMessages は条件に合うメールを取得し、まだ保存していないものを保存する。
//
// 1 通の取得・解読の失敗では止めず、失敗として数えて次へ進む（パイプライン P1・P3 の失敗時の扱い）。
// 認証情報の失効など、続けても無駄な失敗だけでエラーを返す。
func (s *Mail) fetchMessages(ctx context.Context, account *model.MailAccount, search string, backfill bool) (syncOutcome, error) {
	now := s.now()
	outcome := syncOutcome{since: now.AddDate(0, 0, -s.cfg.BackfillDays)}
	if !backfill && account.LastSyncedAt != nil {
		outcome.since = account.LastSyncedAt.Add(-syncOverlap)
	}

	refreshToken, err := s.cfg.Cipher.Decrypt(account.CredentialEncrypted)
	if err != nil {
		return outcome, err
	}
	client := gmail.NewClient(
		s.cfg.OAuth.Client(ctx, &oauth2.Token{RefreshToken: refreshToken}),
		s.cfg.GmailBaseURL,
	)

	// 送信元・件名の条件に、期間を足して検索する（after: は UNIX 時刻の秒でも書ける）
	query := fmt.Sprintf("%s after:%d", search, outcome.since.Unix())

	var refs []gmail.MessageRef
	pageToken := ""
	for {
		page, next, lerr := client.ListMessages(ctx, query, pageToken)
		if lerr != nil {
			return outcome, lerr
		}
		refs = append(refs, page...)
		if next == "" || len(refs) >= maxMessagesPerSync {
			outcome.truncated = next != "" || len(refs) > maxMessagesPerSync
			break
		}
		pageToken = next
	}
	if len(refs) > maxMessagesPerSync {
		refs = refs[:maxMessagesPerSync]
	}

	ids := make([]string, 0, len(refs))
	for _, ref := range refs {
		ids = append(ids, ref.ID)
	}
	existing, err := s.cfg.Messages.ExistingProviderIDs(ctx, account.ID, ids)
	if err != nil {
		return outcome, err
	}

	for _, ref := range refs {
		if existing[ref.ID] {
			continue
		}
		saved, ferr := s.saveMessage(ctx, client, account, ref.ID)
		switch {
		case errors.Is(ferr, gmail.ErrReauthRequired), errors.Is(ferr, gmail.ErrRateLimited),
			errors.Is(ferr, context.DeadlineExceeded):
			// 続けても同じ失敗を繰り返すだけのため、この同期はここで止める。取得できた分は残る
			return outcome, ferr
		case ferr != nil:
			slog.Warn("メールを取り込めませんでした", "mail_account_id", account.ID, "message_id", ref.ID, "error", ferr)
			outcome.failed++
		case saved:
			outcome.fetched++
		}
	}
	return outcome, nil
}

// saveMessage はメール 1 通の原文を取り、分解して保存する。保存したら true。
func (s *Mail) saveMessage(ctx context.Context, client *gmail.Client, account *model.MailAccount, id string) (bool, error) {
	raw, err := client.RawMessage(ctx, id)
	if err != nil {
		return false, err
	}

	parsed, err := mailparse.Parse(raw.Raw)
	if err != nil {
		return false, err
	}

	msg := &model.EmailMessage{
		UserID:            account.UserID,
		MailAccountID:     account.ID,
		ProviderMessageID: raw.ID,
		ThreadID:          optionalString(raw.ThreadID),
		FromAddress:       truncate(parsed.FromAddress),
		FromName:          optionalString(truncate(parsed.FromName)),
		Subject:           optionalString(parsed.Subject),
		ReceivedAt:        raw.ReceivedAt,
		BodyText:          optionalString(parsed.Text),
		BodyHTML:          optionalString(parsed.HTML),
		// 決済メールかの判定と抽出は、テンプレートを作ってから行う（P2・P3）
		Classification: "unknown",
		ParseStatus:    model.ParseStatusPending,
	}
	if msg.FromAddress == "" {
		msg.FromAddress = "unknown"
	}
	return s.cfg.Messages.Insert(ctx, msg)
}

// varcharMax は送信元・店舗名などを入れる列（varchar(255)）の長さ。
const varcharMax = 255

// truncate は varchar(255) の列に入るよう、文字数で切り詰める。
func truncate(s string) string {
	runes := []rune(s)
	if len(runes) <= varcharMax {
		return s
	}
	return string(runes[:varcharMax])
}

// --- 判定・抽出（パイプライン P2〜P4） ---------------------------------------

// SyncBuiltinTemplates は全ユーザ共通のテンプレートを、コードの定義（extract.BuiltinTemplates）に合わせる。
// サーバの起動時に呼ぶ。
func (s *Mail) SyncBuiltinTemplates(ctx context.Context) error {
	templates := make([]model.ParserTemplate, 0, len(extract.BuiltinTemplates))
	for _, b := range extract.BuiltinTemplates {
		subject := b.SubjectPattern
		templates = append(templates, model.ParserTemplate{
			Name:            b.Name,
			FromPattern:     b.FromPattern,
			SubjectPattern:  &subject,
			EventType:       b.EventType,
			BodyFormat:      "text",
			ExtractionRules: b.RulesJSON(),
			Priority:        100,
			IsActive:        true,
			CreatedBy:       "system",
		})
	}
	return s.cfg.Templates.SyncSystem(ctx, templates)
}

// loadTemplates はユーザに使うテンプレートを読み、判定・抽出の準備をする。
// 壊れたテンプレートは飛ばし、ほかのテンプレートでの取込は続ける。
func (s *Mail) loadTemplates(ctx context.Context, userID int64) ([]*extract.Template, error) {
	rows, err := s.cfg.Templates.ListActive(ctx, userID)
	if err != nil {
		return nil, err
	}
	templates := make([]*extract.Template, 0, len(rows))
	for _, row := range rows {
		tmpl, cerr := extract.Compile(extract.TemplateSource{
			ID:              row.ID,
			Name:            row.Name,
			EventType:       row.EventType,
			FromPattern:     row.FromPattern,
			SubjectPattern:  row.SubjectPattern,
			ExtractionRules: row.ExtractionRules,
		})
		if cerr != nil {
			slog.Error("テンプレートを使えません", "parser_template_id", row.ID, "error", cerr)
			continue
		}
		templates = append(templates, tmpl)
	}
	return templates, nil
}

// searchQuery はテンプレートの送信元だけを探す Gmail の検索条件を作る。
//
// 決済と関係の無いメールをサーバに保存しないよう、受信箱全体は取らない（docs/mail-import-mvp.md 判断 2）。
// テンプレートが 1 つも無いときは、設定の仮の条件（件名だけ）で探す。
func searchQuery(templates []*extract.Template, fallback string) string {
	parts := make([]string, 0, len(templates))
	for _, t := range templates {
		if t.GmailQuery != "" {
			parts = append(parts, "("+t.GmailQuery+")")
		}
	}
	if len(parts) == 0 {
		return fallback
	}
	return "(" + strings.Join(parts, " OR ") + ")"
}

// 1 回に読む未処理のメールの数
const processBatch = 200

type processResult int

const (
	processParsed processResult = iota
	processSkipped
	processFailed
)

// processPending は連携の未処理のメールをすべて判定・抽出する。取引を作れた数と、失敗した数を返す。
// データベースの失敗だけで止め、メール 1 通の抽出の失敗では止めない。
func (s *Mail) processPending(ctx context.Context, account *model.MailAccount, templates []*extract.Template) (int, int, error) {
	parsed, failed := 0, 0
	for {
		pending, err := s.cfg.Messages.ListPending(ctx, account.ID, processBatch)
		if err != nil {
			return parsed, failed, err
		}
		if len(pending) == 0 {
			return parsed, failed, nil
		}
		for i := range pending {
			result, perr := s.processMessage(ctx, &pending[i], templates)
			if perr != nil {
				return parsed, failed, perr
			}
			switch result {
			case processParsed:
				parsed++
			case processFailed:
				failed++
			}
		}
	}
}

// processMessage はメール 1 通を判定し、決済なら取引を作る。結果はメールに記録する。
func (s *Mail) processMessage(ctx context.Context, msg *model.EmailMessage, templates []*extract.Template) (processResult, error) {
	now := s.now()
	subject := ""
	if msg.Subject != nil {
		subject = *msg.Subject
	}

	var matched *extract.Template
	for _, t := range templates {
		if t.Matches(msg.FromAddress, subject) {
			matched = t
			break
		}
	}
	if matched == nil {
		// どのテンプレートにも当たらないメールは、決済ではないと決めつけず unknown のまま残す（パイプライン P2）
		return processSkipped, s.cfg.Messages.MarkResult(ctx, msg.ID, map[string]any{
			"classification": model.ClassificationUnknown,
			"classified_at":  now,
			"parse_status":   model.ParseStatusSkipped,
			"parse_error":    "対応するテンプレートがありません",
		})
	}

	body := ""
	if msg.BodyText != nil {
		body = *msg.BodyText
	}
	event, err := matched.Extract(body)
	if skip, isSkip := extract.IsSkip(err); isSkip {
		return processSkipped, s.cfg.Messages.MarkResult(ctx, msg.ID, map[string]any{
			"classification":      model.ClassificationPayment,
			"classified_at":       now,
			"matched_template_id": matched.ID,
			"parse_status":        model.ParseStatusSkipped,
			"parse_error":         skip.Reason,
		})
	}
	if err != nil {
		// テンプレートが合わなくなった（カード会社が通知の形を変えた）可能性がある。
		// 失敗として数え、同期履歴の failed_count で気づけるようにする（パイプライン 0 章「無音の故障」）
		slog.Warn("決済メールから項目を取り出せませんでした", "email_message_id", msg.ID, "template", matched.Name, "error", err)
		return processFailed, s.cfg.Messages.MarkResult(ctx, msg.ID, map[string]any{
			"classification":      model.ClassificationPayment,
			"classified_at":       now,
			"matched_template_id": matched.ID,
			"parse_status":        model.ParseStatusFailed,
			"parse_error":         err.Error(),
		})
	}

	_, err = s.cfg.Messages.SaveExtracted(ctx, repository.ExtractedPayment{
		Email:               msg,
		TemplateID:          matched.ID,
		EventType:           matched.EventType,
		OccurredAt:          event.OccurredAt,
		AmountMinor:         event.AmountMinor,
		Currency:            event.Currency,
		AmountJPYMinor:      event.AmountJPYMinor,
		RawMerchant:         truncate(event.RawMerchant),
		MerchantName:        truncate(event.MerchantName),
		PaymentMethodName:   matched.PaymentMethod.Name,
		PaymentMethodKind:   matched.PaymentMethod.Kind,
		PaymentMethodIssuer: matched.PaymentMethod.Issuer,
		Now:                 now,
	})
	if errors.Is(err, repository.ErrAlreadyProcessed) {
		return processSkipped, nil
	}
	if err != nil {
		return 0, err
	}
	return processParsed, nil
}
