import { RealtimeConnection, type RealtimeHandlers, type SocketLike } from '../connection';

class FakeSocket implements SocketLike {
  static all: FakeSocket[] = [];
  sent: unknown[] = [];
  closed = false;
  onopen: (() => void) | null = null;
  onmessage: ((e: { data: unknown }) => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: (() => void) | null = null;
  constructor(
    public url: string,
    public token: string,
  ) {
    FakeSocket.all.push(this);
  }
  send(data: string) {
    this.sent.push(JSON.parse(data));
  }
  close() {
    this.closed = true;
    this.onclose?.();
  }
  // テストから呼ぶ
  open() {
    this.onopen?.();
  }
  receive(message: object) {
    this.onmessage?.({ data: JSON.stringify(message) });
  }
  drop() {
    this.onclose?.();
  }
}

const latest = () => FakeSocket.all[FakeSocket.all.length - 1]!;

function setup() {
  const handlers: jest.Mocked<RealtimeHandlers> = {
    onSubscribed: jest.fn(),
    onChanged: jest.fn(),
    onRevoked: jest.fn(),
    onRoleChanged: jest.fn(),
  };
  const getToken = jest.fn(async (force: boolean) => (force ? 'fresh' : 'cached'));
  const connection = new RealtimeConnection({
    url: 'ws://api/v1/ws',
    getToken,
    createSocket: (url, token) => new FakeSocket(url, token),
    handlers,
  });
  return { connection, handlers, getToken };
}

// トークンの取得（Promise）を進める
const flush = () => Promise.resolve().then(() => Promise.resolve());

beforeEach(() => {
  jest.useFakeTimers();
  FakeSocket.all = [];
});
afterEach(() => jest.useRealTimers());

describe('RealtimeConnection', () => {
  it('つながったら、所属するすべての世帯を購読する。世帯が変わったら購読し直す', async () => {
    const { connection } = setup();
    connection.setHouseholds(['h1', 'h2']);
    connection.start();
    await flush();
    expect(latest().token).toBe('cached');
    latest().open();
    expect(latest().sent).toEqual([{ type: 'subscribe', householdIds: ['h1', 'h2'] }]);
    connection.setHouseholds(['h1', 'h2']);
    expect(latest().sent).toHaveLength(1);
    connection.setHouseholds(['h1']);
    expect(latest().sent[1]).toEqual({ type: 'subscribe', householdIds: ['h1'] });
    connection.stop();
  });

  it('受け取ったメッセージを、種類ごとに渡す', async () => {
    const { connection, handlers } = setup();
    connection.start();
    await flush();
    latest().open();
    latest().receive({ type: 'subscribed', householdIds: ['h1'] });
    latest().receive({ type: 'changed', householdId: 'h1', version: 2, entity: 'task', entityId: 't1', op: 'updated' });
    latest().receive({ type: 'membership_revoked', householdId: 'h1' });
    latest().receive({ type: 'role_changed', householdId: 'h1', role: 'viewer' });
    expect(handlers.onSubscribed).toHaveBeenCalledWith(['h1']);
    expect(handlers.onChanged).toHaveBeenCalledWith(expect.objectContaining({ entityId: 't1', version: 2 }));
    expect(handlers.onRevoked).toHaveBeenCalledWith('h1');
    expect(handlers.onRoleChanged).toHaveBeenCalledWith('h1', 'viewer');
    connection.stop();
  });

  it('切れたら、1秒・2秒…と間隔を広げてつなぎ直し、つながったら間隔を戻す', async () => {
    const { connection } = setup();
    connection.start();
    await flush();
    latest().open();
    latest().drop();
    expect(FakeSocket.all).toHaveLength(1);
    jest.advanceTimersByTime(1000);
    await flush();
    expect(FakeSocket.all).toHaveLength(2);
    // つながる前に切れた（認証の失敗かもしれない）ので、次はトークンを取り直す
    latest().drop();
    jest.advanceTimersByTime(1999);
    await flush();
    expect(FakeSocket.all).toHaveLength(2);
    jest.advanceTimersByTime(1);
    await flush();
    expect(FakeSocket.all).toHaveLength(3);
    expect(latest().token).toBe('fresh');
    latest().open();
    latest().drop();
    jest.advanceTimersByTime(1000);
    await flush();
    expect(FakeSocket.all).toHaveLength(4);
    connection.stop();
  });

  it('30秒ごとに ping を送り、10秒以内に pong がなければ切ってつなぎ直す', async () => {
    const { connection } = setup();
    connection.start();
    await flush();
    const first = latest();
    first.open();
    jest.advanceTimersByTime(30_000);
    expect(first.sent).toContainEqual({ type: 'ping' });
    first.receive({ type: 'pong' });
    jest.advanceTimersByTime(10_000);
    expect(first.closed).toBe(false);
    jest.advanceTimersByTime(20_000 + 10_000);
    expect(first.closed).toBe(true);
    jest.advanceTimersByTime(1000);
    await flush();
    expect(FakeSocket.all).toHaveLength(2);
    connection.stop();
  });

  it('前面に戻ったとき、切れていれば待たずにつなぎ直す', async () => {
    const { connection } = setup();
    connection.start();
    await flush();
    latest().open();
    latest().drop();
    connection.resume();
    await flush();
    expect(FakeSocket.all).toHaveLength(2);
    // つながっているときは何もしない
    latest().open();
    connection.resume();
    await flush();
    expect(FakeSocket.all).toHaveLength(2);
    connection.stop();
  });

  it('止めたら閉じて、つなぎ直さない', async () => {
    const { connection } = setup();
    connection.start();
    await flush();
    latest().open();
    connection.stop();
    expect(latest().closed).toBe(true);
    jest.advanceTimersByTime(60_000);
    await flush();
    expect(FakeSocket.all).toHaveLength(1);
  });
});
