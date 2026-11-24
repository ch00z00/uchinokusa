import { parseMessage, reconnectDelay, type ChangedMessage } from './messages';

/** WebSocket のうち、使うところだけ（テストで差し替えるため） */
export interface SocketLike {
  send(data: string): void;
  close(): void;
  onopen: (() => void) | null;
  onmessage: ((event: { data: unknown }) => void) | null;
  onclose: (() => void) | null;
  onerror: (() => void) | null;
}

export type RealtimeHandlers = {
  /** 購読できた世帯。ここでデータを取り直す（取り直しと購読の間の変更を逃さないため。0030） */
  onSubscribed: (householdIds: string[]) => void;
  onChanged: (event: ChangedMessage) => void;
  /** 世帯から外された（または所属していない） */
  onRevoked: (householdId: string) => void;
  onRoleChanged: (householdId: string, role: string) => void;
};

type Options = {
  url: string;
  /** ID トークン。forceRefresh は、前の接続がつながる前に切れた（認証の失敗かもしれない）とき */
  getToken: (forceRefresh: boolean) => Promise<string | null>;
  createSocket: (url: string, token: string) => SocketLike;
  handlers: RealtimeHandlers;
};

const pingInterval = 30_000;
const pongTimeout = 10_000;

/**
 * リアルタイム同期の接続（要件11-1・11-5。docs/decisions/0030）。React から切り離して、接続の管理だけを行う。
 * - つながったら、所属するすべての世帯を購読する。世帯が変わったら購読し直す
 * - 切れたら、1秒から倍にしながら（最大30秒）つなぎ直す。つながったら間隔を戻す
 * - React Native からは WebSocket の ping を送れないので、30秒ごとにメッセージの ping を送り、10秒以内に pong がなければ切る
 */
export class RealtimeConnection {
  private socket: SocketLike | null = null;
  private open = false;
  private connecting = false;
  private stopped = true;
  private attempt = 0;
  private refreshToken = false;
  private householdIds: string[] = [];
  private subscribedKey: string | null = null;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private pingTimer: ReturnType<typeof setInterval> | null = null;
  private pongTimer: ReturnType<typeof setTimeout> | null = null;

  constructor(private readonly options: Options) {}

  start(): void {
    if (!this.stopped) return;
    this.stopped = false;
    void this.connect();
  }

  stop(): void {
    this.stopped = true;
    this.clearReconnect();
    this.teardown(true);
  }

  /** 購読する世帯（所属するすべての世帯）。つながっていれば、変わったときだけ購読し直す */
  setHouseholds(ids: readonly string[]): void {
    this.householdIds = [...ids];
    if (this.open) this.sendSubscribe();
  }

  /** アプリが前面に戻ったとき。切れていれば、待たずにつなぎ直す（要件11-5） */
  resume(): void {
    if (this.stopped || this.open || this.connecting) return;
    this.clearReconnect();
    this.attempt = 0;
    void this.connect();
  }

  private async connect(): Promise<void> {
    if (this.stopped) return;
    this.connecting = true;
    let token: string | null = null;
    try {
      token = await this.options.getToken(this.refreshToken);
    } catch {
      token = null;
    }
    this.refreshToken = false;
    if (this.stopped) {
      this.connecting = false;
      return;
    }
    if (!token) {
      this.connecting = false;
      this.scheduleReconnect();
      return;
    }
    const socket = this.options.createSocket(this.options.url, token);
    this.socket = socket;
    socket.onopen = () => {
      if (this.socket !== socket) return;
      this.connecting = false;
      this.open = true;
      this.attempt = 0;
      this.subscribedKey = null;
      this.sendSubscribe();
      this.pingTimer = setInterval(() => this.ping(), pingInterval);
    };
    socket.onmessage = (event) => {
      if (this.socket !== socket || typeof event.data !== 'string') return;
      this.receive(event.data);
    };
    socket.onclose = () => {
      if (this.socket !== socket) return;
      // つながる前に切れたら、トークンの期限切れかもしれないので、次は取り直す
      if (!this.open) this.refreshToken = true;
      this.teardown(false);
      this.scheduleReconnect();
    };
    socket.onerror = () => {
      // エラーのあとは onclose も呼ばれるので、つなぎ直しはそちらで行う
    };
  }

  private receive(raw: string): void {
    const message = parseMessage(raw);
    if (!message) return;
    const { handlers } = this.options;
    switch (message.type) {
      case 'pong':
        this.clearPong();
        return;
      case 'subscribed':
        handlers.onSubscribed(message.householdIds);
        return;
      case 'changed':
        handlers.onChanged(message);
        return;
      case 'membership_revoked':
        handlers.onRevoked(message.householdId);
        return;
      case 'role_changed':
        handlers.onRoleChanged(message.householdId, message.role);
        return;
    }
  }

  private sendSubscribe(): void {
    const key = this.householdIds.join(',');
    if (key === this.subscribedKey) return;
    this.subscribedKey = key;
    this.socket?.send(JSON.stringify({ type: 'subscribe', householdIds: this.householdIds }));
  }

  private ping(): void {
    if (!this.open || this.pongTimer) return;
    this.socket?.send(JSON.stringify({ type: 'ping' }));
    this.pongTimer = setTimeout(() => {
      // 応答がない（通信が途切れている）。閉じる手続きを待たずに切り、つなぎ直す
      this.pongTimer = null;
      this.teardown(true);
      this.scheduleReconnect();
    }, pongTimeout);
  }

  private scheduleReconnect(): void {
    if (this.stopped || this.reconnectTimer) return;
    const delay = reconnectDelay(this.attempt);
    this.attempt += 1;
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      void this.connect();
    }, delay);
  }

  /** 接続を捨てる。close なら閉じる（捨てた接続の onclose は、socket を外してあるので無視される） */
  private teardown(close: boolean): void {
    const socket = this.socket;
    this.socket = null;
    this.open = false;
    this.connecting = false;
    this.subscribedKey = null;
    if (this.pingTimer) clearInterval(this.pingTimer);
    this.pingTimer = null;
    this.clearPong();
    if (close && socket) {
      try {
        socket.close();
      } catch {
        // すでに閉じている
      }
    }
  }

  private clearPong(): void {
    if (this.pongTimer) clearTimeout(this.pongTimer);
    this.pongTimer = null;
  }

  private clearReconnect(): void {
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer);
    this.reconnectTimer = null;
  }
}
