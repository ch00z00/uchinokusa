import { useQueryClient, type QueryClient } from '@tanstack/react-query';
import { router } from 'expo-router';
import type { TFunction } from 'i18next';
import { useEffect, useRef, type ReactNode, type RefObject } from 'react';
import { useTranslation } from 'react-i18next';
import { AppState, Platform } from 'react-native';

import { useAuth } from '@/auth/AuthProvider';
import { authClient } from '@/auth/authClient';
import { useToast, type ToastContextValue } from '@/components/Toast';
import { config } from '@/config';
import { useMe } from '@/features/account/useMe';
import { useSelectedHousehold } from '@/features/household/SelectedHouseholdProvider';
import { fetchMembers, householdKeys, useHouseholds, type Household } from '@/features/household/useHouseholds';
import { normalizeRole } from '@/features/household/permissions';
import { taskTitle } from '@/features/task/taskText';
import type { TodayList } from '@/features/today/todayList';

import { RealtimeConnection, type SocketLike } from './connection';
import { checkVersion, invalidationTargets, type ChangedMessage } from './messages';
import { findTodayItem } from './todayItem';

type NativeWebSocket = new (url: string, protocols: null, options: { headers: Record<string, string> }) => SocketLike;

/** API の URL（…/v1）から、WebSocket の URL（ws(s)://…/v1/ws）を作る */
const websocketUrl = (apiBaseUrl: string) => `${apiBaseUrl.replace(/^http/, 'ws')}/ws`;

/** 世帯のデータすべて（世帯の取得と、植物の詳細）を取り直す。植物の詳細は世帯ごとに分けられないので、すべて */
const refreshHousehold = (queryClient: QueryClient, householdId: string) =>
  Promise.all([
    queryClient.invalidateQueries({ queryKey: ['households', householdId] }),
    queryClient.invalidateQueries({ queryKey: ['plants'] }),
    queryClient.invalidateQueries({ queryKey: householdKeys.all, exact: true }),
  ]);

const todayLists = (queryClient: QueryClient, householdId: string) =>
  queryClient.getQueriesData<TodayList>({ queryKey: ['households', householdId, 'today'] }).map(([, data]) => data);

type Latest = {
  meId: string | undefined;
  households: Household[] | undefined;
  selectedId: string;
  t: TFunction;
  toast: ToastContextValue;
};

/**
 * 接続と、受け取ったときの処理を作る。処理は接続を作ったときに1度だけ渡すので、
 * 画面の最新の値（自分・世帯・文言・トースト）は latest から読む
 */
function createConnection(queryClient: QueryClient, latest: RefObject<Latest>): RealtimeConnection {
  // 世帯ごとに、最後に受け取ったバージョン番号
  const versions = new Map<string, number>();
  const householdName = (id: string) => latest.current.households?.find((h) => h.id === id)?.name ?? '';

  /** ほかのメンバーが今日やることを済ませたら、取り直したあとに一覧から消えたのを確かめて、誰が対応したかを知らせる（要件9-7） */
  const announceHandled = async (event: ChangedMessage, refreshed: Promise<unknown>) => {
    const { meId } = latest.current;
    if (event.entity !== 'task' || event.op !== 'updated' || !event.actorUserId || event.actorUserId === meId) return;
    const before = findTodayItem(todayLists(queryClient, event.householdId), event.entityId);
    if (!before) return;
    await refreshed;
    if (findTodayItem(todayLists(queryClient, event.householdId), event.entityId)) return;
    let name: string | undefined;
    try {
      const members = await queryClient.fetchQuery({
        queryKey: householdKeys.members(event.householdId),
        queryFn: () => fetchMembers(event.householdId),
      });
      name = members.find((m) => m.userId === event.actorUserId)?.displayName;
    } catch {
      name = undefined;
    }
    const { t, toast } = latest.current;
    toast.show(
      t('realtime.handled', {
        name: name ?? t('realtime.someone'),
        plant: before.plant.name,
        task: taskTitle(before.task, t),
      }),
    );
  };

  return new RealtimeConnection({
    url: websocketUrl(config.apiBaseUrl),
    getToken: (forceRefresh) => authClient.getIdToken(forceRefresh),
    // React Native の WebSocket は、3つめの引数で接続のヘッダーを付けられる（0030 の決まり：Authorization で認証）。
    // 実装（react-native/Libraries/WebSocket）にはあるが、型の定義にないので、型だけ合わせる
    createSocket: (url, token) =>
      new (WebSocket as unknown as NativeWebSocket)(url, null, { headers: { Authorization: `Bearer ${token}` } }),
    handlers: {
      // 購読できたら、その世帯のデータを取り直す（切れていたあいだの変更を取り込む。要件11-5）
      onSubscribed: (ids) => {
        for (const id of ids) {
          versions.delete(id);
          void refreshHousehold(queryClient, id);
        }
      },
      onChanged: (event) => {
        const result = checkVersion(versions.get(event.householdId), event.version);
        if (result === 'stale') return;
        versions.set(event.householdId, event.version);
        // 番号が飛んだら取りこぼしがあるので、世帯のデータをすべて取り直す
        const refreshed =
          result === 'gap'
            ? refreshHousehold(queryClient, event.householdId)
            : Promise.all(invalidationTargets(event).map((target) => queryClient.invalidateQueries(target)));
        void announceHandled(event, refreshed);
      },
      onRevoked: (householdId) => {
        const name = householdName(householdId);
        const wasSelected = latest.current.selectedId === householdId;
        versions.delete(householdId);
        queryClient.removeQueries({ queryKey: ['households', householdId] });
        void queryClient.invalidateQueries({ queryKey: householdKeys.all, exact: true });
        latest.current.toast.show(latest.current.t('realtime.revoked', { household: name }));
        // 表示中の世帯から外されたら、開いている画面を閉じて今日へ戻る。ほかの世帯がなければ、はじめての選択へ移る
        if (wasSelected) router.dismissTo('/today');
      },
      onRoleChanged: (householdId, role) => {
        // ロールは世帯の一覧から出しているので、取り直すと操作の出し分けが変わる（要件4-6）
        void queryClient.invalidateQueries({ queryKey: householdKeys.all, exact: true });
        void queryClient.invalidateQueries({ queryKey: householdKeys.members(householdId) });
        const { t, toast } = latest.current;
        toast.show(
          t('realtime.roleChanged', { household: householdName(householdId), role: t(`roles.${normalizeRole(role)}`) }),
        );
      },
    },
  });
}

/**
 * リアルタイム同期（タスク 18.1、要件11-1・11-5・9-7・4-6・4-7）。サインインしているあいだ、WebSocket で
 * 所属するすべての世帯を購読し、変更を受けたら該当するクエリを取り直す。
 * ほかのメンバーが今日やることを済ませたら、トーストで誰が対応したかを知らせる。
 * ロールの変更・世帯から外されたことも、トーストで知らせ、表示を更新する
 */
export function RealtimeProvider({ children }: { children: ReactNode }) {
  const auth = useAuth();
  const queryClient = useQueryClient();
  const households = useHouseholds();
  const me = useMe();
  const toast = useToast();
  const { t } = useTranslation();
  const { state } = useSelectedHousehold();
  const latest = useRef<Latest>({ meId: undefined, households: undefined, selectedId: '', t, toast });
  const connectionRef = useRef<RealtimeConnection | null>(null);
  const householdIds = households.data?.map((h) => h.id).join(',') ?? '';
  const selectedId = state.status === 'selected' ? state.household.id : '';

  useEffect(() => {
    latest.current = { meId: me.data?.id, households: households.data, selectedId, t, toast };
  }, [me.data?.id, households.data, selectedId, t, toast]);

  const signedIn = auth.status === 'signedIn';
  useEffect(() => {
    // ブラウザの WebSocket は接続のヘッダーを付けられないので、いまはつながない（0030 の「最初のメッセージでトークン」を足すまで）
    if (!signedIn || Platform.OS === 'web') return;
    const connection = createConnection(queryClient, latest);
    connectionRef.current = connection;
    connection.start();
    // 前面に戻ったとき、切れていれば待たずにつなぎ直す（データの取り直しは、TanStack Query の画面に戻ったときの取り直しと、購読できたときに行う）
    const subscription = AppState.addEventListener('change', (status) => {
      if (status === 'active') connection.resume();
    });
    return () => {
      subscription.remove();
      connection.stop();
      connectionRef.current = null;
    };
  }, [queryClient, signedIn]);

  // 購読する世帯（所属するすべての世帯）。接続を作り直したときにも渡す
  useEffect(() => {
    connectionRef.current?.setHouseholds(householdIds ? householdIds.split(',') : []);
  }, [householdIds, signedIn]);

  return children;
}
