package extract_test

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/extract"
)

// TestBuiltinTemplates_手元の実メール は、取り込んだ実際のメールで判定と抽出を試す。
//
// scripts/dump-mail-samples.sh が書き出す mail-samples/messages.jsonl があるときだけ動く。
// 個人のメールのためリポジトリには無く、CI では常にスキップされる。
// ルールを変えたときに、手元の実メールで取りこぼしが増えていないかを確かめるために使う。
// 出力は件数と理由だけにし、メールの中身は出さない。
//
//	go test ./internal/extract -run 実メール -v
func TestBuiltinTemplates_手元の実メール(t *testing.T) {
	path := filepath.Join("..", "..", "..", "mail-samples", "messages.jsonl")
	file, err := os.Open(path)
	if err != nil {
		t.Skip("mail-samples/messages.jsonl が無いためスキップします")
	}
	defer func() { _ = file.Close() }()

	templates := make([]*extract.Template, 0, len(extract.BuiltinTemplates))
	for _, b := range extract.BuiltinTemplates {
		tmpl, cerr := extract.Compile(b.Source())
		if cerr != nil {
			t.Fatalf("テンプレートを組み立てられません: %v", cerr)
		}
		templates = append(templates, tmpl)
	}

	results := map[string]int{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 1<<20), 16<<20)
	for scanner.Scan() {
		var mail struct {
			FromAddress string `json:"from_address"`
			Subject     string `json:"subject"`
			BodyText    string `json:"body_text"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &mail); err != nil {
			t.Fatalf("1 行を読めません: %v", err)
		}

		var matched *extract.Template
		for _, tmpl := range templates {
			if tmpl.Matches(mail.FromAddress, mail.Subject) {
				matched = tmpl
				break
			}
		}
		if matched == nil {
			results["（テンプレートに一致しない）"]++
			continue
		}

		_, xerr := matched.Extract(mail.BodyText)
		switch skip, isSkip := extract.IsSkip(xerr); {
		case xerr == nil:
			results[matched.Name+" / 取り込む"]++
		case isSkip:
			results[matched.Name+" / 取り込まない: "+skip.Reason]++
		default:
			results[matched.Name+" / 失敗: "+xerr.Error()]++
			t.Errorf("%s: 抽出に失敗しました: %v", matched.Name, xerr)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("読み込みに失敗しました: %v", err)
	}

	keys := make([]string, 0, len(results))
	for k := range results {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		t.Logf("%4d  %s", results[k], k)
	}
}
