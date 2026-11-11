// Step 8 の提出物「動作確認」のブラウザ操作のスクリーンショットを撮る。
// docs/step8-submission.md ③ の表のとおりに、公開 URL の画面を実際に操作して撮る。
// Step 6 の撮影（docs/step6-submission.md ②）と同じやり方で、ヘッドレスの Chrome を使う。
//
// 直接は呼ばず、scripts/take-screenshots.sh から呼ぶ（puppeteer-core の用意と環境変数の受け渡し）。
//
// - 撮影の前に、画面上のメールアドレスを伏せ字にする（ヘッダーの利用者名・メール連携の Gmail）
// - 画面の操作で呼ばれた API（メソッド・パス・ステータス）を api-log.txt に書く。トークンは書かない
// - テスト用に作った取引・カテゴリは、途中で失敗しても最後に消す
import fs from "node:fs";
import { createRequire } from "node:module";

const BASE = process.env.BASE_URL ?? "https://app.autokakeibo.com";
const OUT = process.env.OUT_DIR;
const TOKEN_FILE = process.env.TOKEN_FILE;
const CHROME = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome";
const DESKTOP = { width: 1280, height: 860, deviceScaleFactor: 2 };

// puppeteer-core はプロジェクトの依存に入れず、PUPPETEER_DIR に入れたものを使う
const puppeteer = createRequire(`${process.env.PUPPETEER_DIR}/`)("puppeteer-core");

if (!OUT || !TOKEN_FILE) throw new Error("OUT_DIR と TOKEN_FILE を指定してください");
const token = fs.readFileSync(TOKEN_FILE, "utf8").trim();
fs.mkdirSync(OUT, { recursive: true });

const browser = await puppeteer.launch({ executablePath: CHROME, headless: true, defaultViewport: DESKTOP });
const page = await browser.newPage();
page.setDefaultTimeout(20000);

// 画面の操作で呼ばれた API の記録。phase は今どの操作をしているか
let phase = "";
const apiLog = [];
page.on("response", (res) => {
  const url = new URL(res.url());
  if (url.origin !== new URL(BASE).origin) return;
  if (!url.pathname.startsWith("/api/") && url.pathname !== "/health") return;
  apiLog.push(`${phase}\t${res.request().method()} ${url.pathname}${url.search}\t${res.status()}`);
});

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const goto = (path) => page.goto(BASE + path, { waitUntil: "networkidle0" });
const waitFor = (fn, ...args) => page.waitForFunction(fn, {}, ...args);
const waitText = (text) => waitFor((t) => document.body.innerText.includes(t), text);

// 画面上のメールアドレスを「●●●●@ドメイン」にする（スクリーンショットに個人のアドレスを写さない）
const maskEmails = () =>
  page.evaluate(() => {
    const re = /[A-Za-z0-9._%+-]+@([A-Za-z0-9.-]+\.[A-Za-z]{2,})/g;
    const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
    for (let node = walker.nextNode(); node; node = walker.nextNode()) {
      if (re.test(node.nodeValue)) node.nodeValue = node.nodeValue.replace(re, "●●●●@$1");
      re.lastIndex = 0;
    }
  });

const shot = async (name, fullPage = false) => {
  // マウスを端へ寄せ、行のホバーの色を写さない
  await page.mouse.move(1, 1);
  await maskEmails();
  await sleep(300);
  await page.screenshot({ path: `${OUT}/${name}`, fullPage });
  console.log("撮影:", name);
};

// 見えているボタンを文言で押す。root を渡すと、その中だけを探す
const clickButton = (text, root = "body") =>
  page.evaluate(
    (t, r) => {
      const scope = document.querySelector(r);
      const button = [...scope.querySelectorAll("button")].find(
        (b) => b.textContent.trim() === t && b.getClientRects().length > 0,
      );
      if (!button) throw new Error(`ボタンが見つかりません: ${t}`);
      button.click();
    },
    text,
    root,
  );
const listLoaded = () =>
  waitFor(() => document.querySelector('main [aria-busy="false"]') && document.querySelector("main ul li a"));

// 画面の外から API を呼ぶ（テスト用データの準備と片付け）。ログインした状態のページで使う
const api = (method, path, body) =>
  page.evaluate(
    async (m, p, b) => {
      const t = sessionStorage.getItem("kakeibo.access_token");
      const headers = { Authorization: `Bearer ${t}`, Accept: "application/json" };
      if (b !== null) headers["Content-Type"] = "application/json";
      const res = await fetch(`/api/v1${p}`, { method: m, headers, body: b === null ? undefined : JSON.stringify(b) });
      if (!res.ok) throw new Error(`${m} ${p}: ${res.status} ${await res.text()}`);
      return res.status === 204 ? null : (await res.json()).data;
    },
    method,
    path,
    body ?? null,
  );

// 一覧が空だと画面の確認にならないため、手入力の取引 3 件をテスト用に作り、撮ったあと消す
const today = new Date(Date.now() + 9 * 3600e3).toISOString().slice(0, 10); // JST の今日
const TEST_CATEGORY = "確認用カテゴリ";
const setupTransactionIds = [];
let createdTransactionId = null;

const setup = async () => {
  const categories = await api("GET", "/categories");
  const byName = (name) => categories.find((c) => c.name === name)?.id ?? null;
  const cash = (await api("GET", "/payment-methods")).find((p) => p.kind === "cash")?.id;
  const items = [
    { time: "08:30", amount: 2380, merchant: "近所のスーパー", category: byName("食費") },
    { time: "10:40", amount: 980, merchant: "ドラッグストア", category: byName("日用品") },
    { time: "12:15", amount: 950, merchant: "駅前のラーメン屋", category: byName("食費") },
  ];
  for (const item of items) {
    const txn = await api("POST", "/transactions", {
      occurred_at: `${today}T${item.time}:00+09:00`,
      amount_minor: item.amount,
      merchant_name: item.merchant,
      category_id: item.category,
      ...(cash ? { payment_method_id: cash } : {}),
    });
    setupTransactionIds.push(txn.id);
  }
  console.log("準備: 取引", setupTransactionIds.join(", "));
};

try {
  // 1. ログイン（ログインしていない状態）
  phase = "01 ログイン";
  await goto("/login");
  await waitText("Google でログイン");
  await shot("01-login.png");

  // ここからログインした状態。トークンを sessionStorage に置く（ログイン処理中の画面と同じ置き場所）
  await page.evaluate((t) => sessionStorage.setItem("kakeibo.access_token", t), token);
  phase = "準備";
  await setup();

  // 2. 取引一覧
  phase = "02 取引一覧";
  await goto("/transactions");
  await listLoaded();
  await shot("02-list.png", true);

  // 3. 取引登録（入力したところ）
  phase = "03 取引登録";
  await goto("/transactions/new");
  await waitFor(() => document.querySelector("form button[type=submit]:not([disabled])"));
  await page.type('form input[inputmode="numeric"]', "1200");
  await page.type('input[placeholder="例: 近所の定食屋"]', "近所の定食屋");
  await shot("03-new.png");

  // 4. 登録のあと（完了メッセージは 4 秒で消えるため、出たらすぐ撮る）
  await page.click("form button[type=submit]");
  await waitFor(
    () =>
      location.pathname === "/transactions" &&
      document.querySelector("[role=status]") &&
      document.querySelector('main [aria-busy="false"]') &&
      document.body.innerText.includes("近所の定食屋"),
  );
  await shot("04-list-created.png");

  // 5. 取引詳細（登録した取引）
  phase = "05 取引詳細";
  await page.evaluate(() => {
    const link = [...document.querySelectorAll("main ul li a")].find((a) => a.innerText.includes("近所の定食屋"));
    link.click();
  });
  await waitFor(() => /^\/transactions\/\d+$/.test(location.pathname) && document.querySelector("main dl"));
  createdTransactionId = await page.evaluate(() => location.pathname.split("/").pop());
  await shot("05-detail.png");

  // 6. 取引編集（カテゴリを変えたところ。変えると「保存」を押せるようになる）
  phase = "06 取引編集";
  await page.evaluate(() => {
    const edit = [...document.querySelectorAll('a[href$="/edit"]')].find((a) => a.getClientRects().length > 0);
    edit.click();
  });
  await waitFor(
    () =>
      location.pathname.endsWith("/edit") &&
      document.querySelector('form input[inputmode="numeric"]')?.value === "1200" &&
      document.querySelector("form button[type=submit]")?.disabled,
  );
  const category = await page.evaluate(() => {
    const options = [...document.querySelector("form select").options].filter((o) => o.value);
    return (options.find((o) => o.textContent.trim() === "食費") ?? options[0]).value;
  });
  await page.select("form select", category);
  await waitFor(() => !document.querySelector("form button[type=submit]")?.disabled);
  await shot("06-edit.png");

  // 7. 保存のあと
  await page.click("form button[type=submit]");
  await waitFor(
    () =>
      /^\/transactions\/\d+$/.test(location.pathname) &&
      document.querySelector("[role=status]")?.innerText.includes("保存しました"),
  );
  await shot("07-detail-saved.png");

  // 8・9. 削除の確認と、削除のあと
  phase = "08 取引削除";
  await clickButton("削除");
  await waitFor(() => document.querySelector("dialog[open]"));
  await shot("08-detail-delete.png");
  await clickButton("削除する", "dialog[open]");
  await waitFor(
    () =>
      location.pathname === "/transactions" &&
      document.querySelector("[role=status]") &&
      document.querySelector('main [aria-busy="false"]'),
  );
  createdTransactionId = null;
  await shot("09-list-deleted.png");

  // 10. カテゴリ管理（画面から追加したところ）
  phase = "10 カテゴリ管理";
  await goto("/settings/categories");
  await waitFor(() => document.querySelector("tbody tr"));
  await clickButton("＋ 追加");
  await waitFor(() => document.querySelector("dialog[open] input"));
  await page.type('dialog[open] input[placeholder="例: 交際費"]', TEST_CATEGORY);
  await clickButton("保存", "dialog[open]");
  await waitFor(
    (name) => !document.querySelector("dialog[open]") && document.querySelector("tbody")?.innerText.includes(name),
    TEST_CATEGORY,
  );
  await shot("10-categories.png");

  // 11. メール連携（連携していれば、その状態。アドレスは伏せ字）
  phase = "11 メール連携";
  await goto("/settings/mail-accounts");
  await waitFor(
    () => document.body.innerText.includes("今すぐ同期") || document.body.innerText.includes("連携しているメールはありません"),
  );
  await shot("11-mail-accounts.png");
} catch (error) {
  console.error("失敗しました:", error.message);
  await maskEmails().catch(() => {});
  await page.screenshot({ path: `${OUT}/_error.png` }).catch(() => {});
  process.exitCode = 1;
} finally {
  // テスト用に作った取引・カテゴリを、途中で失敗していても片付ける
  phase = "片付け";
  await goto("/transactions").catch(() => {});
  const done = [];
  for (const id of [...setupTransactionIds, createdTransactionId].filter(Boolean)) {
    await api("DELETE", `/transactions/${id}`)
      .then(() => done.push(`取引 ${id}`))
      .catch((e) => done.push(`取引 ${id} の削除に失敗: ${e.message}`));
  }
  const testCategory = await api("GET", "/categories")
    .then((list) => list.find((c) => c.name === TEST_CATEGORY))
    .catch(() => null);
  if (testCategory) {
    await api("DELETE", `/categories/${testCategory.id}`)
      .then(() => done.push(TEST_CATEGORY))
      .catch((e) => done.push(`${TEST_CATEGORY} の削除に失敗: ${e.message}`));
  }
  console.log("片付け:", done.length ? done.join(", ") : "なし");

  const header = [
    `# ${BASE} の画面を操作したときに呼ばれた API（scripts/take-screenshots.mjs、${new Date().toISOString()}）`,
    "# 操作\tメソッド パス\tステータス",
  ];
  fs.writeFileSync(`${OUT}/api-log.txt`, [...header, ...apiLog].join("\n") + "\n");
  console.log("API の記録:", `${OUT}/api-log.txt（${apiLog.length} 件）`);
  await browser.close();
}
