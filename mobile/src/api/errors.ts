import type { components } from './schema';

type ErrorBody = components['schemas']['Error'];

/** サーバーが返すエラーの種類（api/openapi.yaml の ErrorCode） */
export type ServerErrorCode = ErrorBody['code'];

/**
 * アプリで扱うエラーの種類。サーバーの code に、次の2つを足したもの。
 * - `network`：サーバーに届かなかった（通信できない、タイムアウトなど）
 * - `unknown`：アプリが知らない code（サーバーが新しい code を足したとき）や、形の合わないレスポンス
 */
export type ApiErrorCode = ServerErrorCode | 'network' | 'unknown';

// 実行時に code を判定するための一覧。Record にすることで、ErrorCode に値が増えたら型エラーで気づける
const serverErrorCodes: Record<ServerErrorCode, true> = {
  bad_request: true,
  validation_failed: true,
  unauthenticated: true,
  forbidden: true,
  not_found: true,
  conflict: true,
  last_owner: true,
  already_member: true,
  already_completed: true,
  stale_cycle: true,
  undo_expired: true,
  already_responded: true,
  suggestion_expired: true,
  owner_transfer_required: true,
  invitation_invalid: true,
  rate_limited: true,
  internal: true,
};

export const apiErrorCodes = [
  ...(Object.keys(serverErrorCodes) as ServerErrorCode[]),
  'network',
  'unknown',
] as const satisfies readonly ApiErrorCode[];

function isServerErrorCode(value: unknown): value is ServerErrorCode {
  return typeof value === 'string' && Object.prototype.hasOwnProperty.call(serverErrorCodes, value);
}

/**
 * API の呼び出しの失敗。画面は `code` で分岐し、`message` は表示しない（表示する文言は i18n で引く）。
 */
export class ApiError extends Error {
  /** HTTP のステータス。サーバーに届かなかったときは null */
  readonly status: number | null;
  readonly code: ApiErrorCode;
  /** 入力欄ごとのエラーの理由（validation_failed のとき） */
  readonly fields: Readonly<Record<string, string>>;
  /** レスポンスの本文。衝突の内容（誰がいつ完了したか、など）を読むときに使う */
  readonly body: unknown;

  constructor(params: {
    status: number | null;
    code: ApiErrorCode;
    message: string;
    fields?: Record<string, string>;
    body?: unknown;
    cause?: unknown;
  }) {
    super(params.message, { cause: params.cause });
    this.name = 'ApiError';
    this.status = params.status;
    this.code = params.code;
    this.fields = params.fields ?? {};
    this.body = params.body;
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function toFields(value: unknown): Record<string, string> {
  if (!isRecord(value)) return {};
  return Object.fromEntries(
    Object.entries(value).filter((entry): entry is [string, string] => typeof entry[1] === 'string'),
  );
}

/** エラーのレスポンス（ステータスと本文）を ApiError に正規化する */
export function toApiError(status: number, body: unknown): ApiError {
  if (!isRecord(body)) {
    return new ApiError({ status, code: 'unknown', message: `HTTP ${status}`, body });
  }
  return new ApiError({
    status,
    code: isServerErrorCode(body.code) ? body.code : 'unknown',
    message: typeof body.message === 'string' ? body.message : `HTTP ${status}`,
    fields: toFields(body.fields),
    body,
  });
}

/** サーバーに届かなかったこと（fetch が投げたエラー）を ApiError に正規化する */
export function toNetworkError(cause: unknown): ApiError {
  const message = cause instanceof Error ? cause.message : 'network error';
  return new ApiError({ status: null, code: 'network', message, cause });
}
