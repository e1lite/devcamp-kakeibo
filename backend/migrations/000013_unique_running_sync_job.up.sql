-- 同じ連携の同期が同時に 2 つ実行中にならないよう、DB で防ぐ（docs/step7-infra.md 6 章 #4）。
--
-- これまでは API サーバのプロセス内のロックで防いでいたが、API サーバが複数台だと、
-- 2 台に同時に「今すぐ同期」が届いたときに台をまたいでは効かない。
-- 部分 UNIQUE 索引なら、実行中（running）の行は連携ごとに 1 行しか入らない。

-- 索引を作る前に、実行中のまま重複している古い行を失敗にしておく（作成時の UNIQUE 違反を防ぐ）
UPDATE sync_jobs
SET status = 'failed',
    finished_at = now(),
    error_message = '同じ連携の同期が重複して実行中のまま残っていたため、失敗にしました'
WHERE status = 'running'
  AND id NOT IN (
      SELECT max(id) FROM sync_jobs WHERE status = 'running' GROUP BY mail_account_id
  );

CREATE UNIQUE INDEX uq_sync_jobs_running ON sync_jobs (mail_account_id) WHERE status = 'running';
