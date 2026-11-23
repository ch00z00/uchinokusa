#!/usr/bin/env node
// README.md の進捗の区間（<!-- progress:start --> 〜 <!-- progress:end -->）を書き直す。
// 進捗は tasks.md と docs/roadmap.md のチェックボックスの数から、期間は docs/progress/schedule.json から計算する。
// 使い方：node scripts/update-progress.mjs [--today YYYY-MM-DD]
// pre-commit のフック（.githooks/pre-commit）から、tasks.md・roadmap.md・schedule.json を含むコミットで呼ばれる。
import { readFileSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = join(dirname(fileURLToPath(import.meta.url)), '..');
const read = (path) => readFileSync(join(root, path), 'utf8');

// 期間の経過より、このポイント以上遅れていたら「遅れ気味」にする
const behindThreshold = 20;
const barWidth = 10;

/** 端末のタイムゾーンでの今日（YYYY-MM-DD） */
function localToday() {
  const d = new Date();
  const pad = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

const dayNumber = (date) => Date.UTC(...date.split('-').map((v, i) => (i === 1 ? Number(v) - 1 : Number(v)))) / 86400000;
const shortDate = (date) => date.slice(5).replace('-', '/');

/**
 * tasks.md の、章（1. など）の範囲のチェックボックスを数える。サブタスク（1.1 など）があればサブタスクを、
 * なければ章そのものを数える
 */
export function countTasks(text, [from, to]) {
  const chapters = new Map();
  for (const line of text.split('\n')) {
    const m = /^- \[( |x)\] (\d+)(?:\.(\d+))?[ .]/.exec(line);
    if (!m) continue;
    const chapter = Number(m[2]);
    if (chapter < from || chapter > to) continue;
    const entry = chapters.get(chapter) ?? { own: { done: 0, total: 0 }, subs: { done: 0, total: 0 } };
    const bucket = m[3] === undefined ? entry.own : entry.subs;
    bucket.total += 1;
    if (m[1] === 'x') bucket.done += 1;
    chapters.set(chapter, entry);
  }
  let done = 0;
  let total = 0;
  for (const { own, subs } of chapters.values()) {
    const counted = subs.total > 0 ? subs : own;
    done += counted.done;
    total += counted.total;
  }
  return { done, total };
}

/** roadmap.md の見出し（## 名前）の下のチェックボックスを数える */
export function countSection(text, section) {
  let inside = false;
  let done = 0;
  let total = 0;
  for (const line of text.split('\n')) {
    if (line.startsWith('## ')) {
      inside = line.slice(3).trim() === section;
      continue;
    }
    if (!inside) continue;
    const m = /^- \[( |x)\] /.exec(line);
    if (!m) continue;
    total += 1;
    if (m[1] === 'x') done += 1;
  }
  return { done, total };
}

const percent = (done, total) => (total === 0 ? 0 : Math.round((done / total) * 100));
const bar = (pct) => {
  const filled = Math.round((pct / 100) * barWidth);
  return '█'.repeat(filled) + '░'.repeat(barWidth - filled);
};

/** 期間のうち、今日までに過ぎた割合（%） */
function elapsedPercent(start, end, today) {
  const length = dayNumber(end) - dayNumber(start) + 1;
  const passed = dayNumber(today) - dayNumber(start) + 1;
  return Math.round((Math.min(Math.max(passed, 0), length) / length) * 100);
}

function statusOf({ pct, elapsed, start, end }, today) {
  if (pct === 100) return { label: '✅ 完了', tag: 'done' };
  if (today < start) return { label: '⏳ 開始前', tag: '' };
  if (today > end) return { label: '🔴 期限切れ', tag: 'crit' };
  if (elapsed - pct >= behindThreshold) return { label: '🔴 遅れ気味', tag: 'crit, active' };
  return { label: '🟢 順調', tag: 'active' };
}

export function render(schedule, files, today) {
  const rows = schedule.phases.map((phase) => {
    let done = 0;
    let total = 0;
    for (const source of phase.sources) {
      const count = source.tasks
        ? countTasks(files[source.tasks], source.chapters)
        : countSection(files[source.roadmap], source.section);
      done += count.done;
      total += count.total;
    }
    const pct = percent(done, total);
    const elapsed = elapsedPercent(phase.start, phase.end, today);
    return { ...phase, done, total, pct, elapsed, status: statusOf({ pct, elapsed, ...phase }, today) };
  });
  const done = rows.reduce((sum, r) => sum + r.done, 0);
  const total = rows.reduce((sum, r) => sum + r.total, 0);
  const daysLeft = (date) => dayNumber(date) - dayNumber(today);
  const milestones = (schedule.milestones ?? [])
    .filter((m) => m.date >= today)
    .map((m) => `${m.name}（${m.date}）まで **あと ${daysLeft(m.date)} 日**`);

  const lines = [
    '<!-- progress:start -->',
    `### 進捗（${today} 時点・自動更新）`,
    '',
    [
      `**全体 ${percent(done, total)}%（${done}/${total}）**`,
      ...milestones,
      `締め切り（${schedule.deadline}）まで **あと ${daysLeft(schedule.deadline)} 日**`,
    ].join('　／　'),
    '',
    '| フェーズ | 期間 | 進捗 | 期間の経過 | 状態 |',
    '| --- | --- | --- | --- | --- |',
    ...rows.map(
      (r) =>
        `| ${r.name} | ${shortDate(r.start)}〜${shortDate(r.end)} | \`${bar(r.pct)}\` ${r.pct}%（${r.done}/${r.total}） | ${r.elapsed}% | ${r.status.label} |`,
    ),
    '',
    '```mermaid',
    'gantt',
    '    title スケジュール（赤：遅れ／濃い色：進行中）',
    '    dateFormat YYYY-MM-DD',
    '    axisFormat %m/%d',
    '    section フェーズ',
    ...rows.map((r) => {
      const tag = r.status.tag ? `${r.status.tag}, ` : '';
      return `    ${r.gantt} ${r.pct}% :${tag}${r.id}, ${r.start}, ${r.end}`;
    }),
    ...(schedule.milestones ?? []).map((m, i) => `    ${m.name} :milestone, m${i}, ${m.date}, 0d`),
    `    締め切り :milestone, deadline, ${schedule.deadline}, 0d`,
    '```',
    '',
    `> 進捗は [tasks.md](.kiro/specs/household-plant-care-mvp/tasks.md) と [docs/roadmap.md](docs/roadmap.md) のチェックボックスの数から計算している（作業量の重みは付けていない）。期間の経過より ${behindThreshold} ポイント以上遅れると「遅れ気味」。期間は [docs/progress/schedule.json](docs/progress/schedule.json)。`,
    '<!-- progress:end -->',
  ];
  return lines.join('\n');
}

function main() {
  const todayArg = process.argv.indexOf('--today');
  const today = todayArg > 0 ? process.argv[todayArg + 1] : localToday();
  const schedule = JSON.parse(read('docs/progress/schedule.json'));
  const paths = new Set(schedule.phases.flatMap((p) => p.sources.map((s) => s.tasks ?? s.roadmap)));
  const files = Object.fromEntries([...paths].map((p) => [p, read(p)]));
  const readme = read('README.md');
  const block = /<!-- progress:start -->[\s\S]*?<!-- progress:end -->/;
  if (!block.test(readme)) {
    console.error('README.md に <!-- progress:start --> 〜 <!-- progress:end --> がありません');
    process.exit(1);
  }
  const next = readme.replace(block, render(schedule, files, today));
  if (next !== readme) writeFileSync(join(root, 'README.md'), next);
}

if (process.argv[1] === fileURLToPath(import.meta.url)) main();
