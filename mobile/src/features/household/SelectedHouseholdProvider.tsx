import { createContext, use, useEffect, useState, type ReactNode } from 'react';

import { getSelectedHouseholdId, setSelectedHouseholdId } from '@/lib/deviceStorage';

import { can, type Permission } from './permissions';
import { resolveSelectedHouseholdId } from './selection';
import { useHouseholds, type Household } from './useHouseholds';

export type SelectedHouseholdState =
  | { status: 'loading' }
  | { status: 'error'; error: unknown; retry: () => void }
  /** どの世帯にも所属していない（はじめての選択 ONB-01 を出す） */
  | { status: 'none' }
  | { status: 'selected'; household: Household };

type ContextValue = {
  state: SelectedHouseholdState;
  /** 表示する世帯を切り替え、端末に保存する（要件2-4） */
  select: (householdId: string) => void;
};

const SelectedHouseholdContext = createContext<ContextValue | null>(null);

export function SelectedHouseholdProvider({ children }: { children: ReactNode }) {
  const households = useHouseholds();
  // 端末に保存した世帯。読み込むまでは undefined
  const [storedId, setStoredId] = useState<string | null | undefined>(undefined);

  useEffect(() => {
    let active = true;
    void getSelectedHouseholdId().then((id) => {
      if (active) setStoredId((current) => (current === undefined ? id : current));
    });
    return () => {
      active = false;
    };
  }, []);

  const select = (householdId: string) => {
    setStoredId(householdId);
    void setSelectedHouseholdId(householdId);
  };

  return (
    <SelectedHouseholdContext value={{ state: toState(households, storedId), select }}>
      {children}
    </SelectedHouseholdContext>
  );
}

function toState(
  households: ReturnType<typeof useHouseholds>,
  storedId: string | null | undefined,
): SelectedHouseholdState {
  if (households.data === undefined) {
    if (households.isError) return { status: 'error', error: households.error, retry: () => void households.refetch() };
    return { status: 'loading' };
  }
  if (storedId === undefined) return { status: 'loading' };
  // 保存した世帯から外れていたら（退出・削除・外された）、最初の世帯にする
  const id = resolveSelectedHouseholdId(households.data, storedId);
  const household = households.data.find((h) => h.id === id);
  return household ? { status: 'selected', household } : { status: 'none' };
}

export function useSelectedHousehold(): ContextValue {
  const value = use(SelectedHouseholdContext);
  if (!value) throw new Error('SelectedHouseholdProvider の中で使う');
  return value;
}

/**
 * 選択中の世帯で、操作を出してよいか（要件4-5）。閲覧者には編集・記録の操作を出さない。
 * 世帯が決まっていないあいだは false（操作を出さない）
 */
export function useCan(permission: Permission): boolean {
  const { state } = useSelectedHousehold();
  return state.status === 'selected' && can(state.household.role, permission);
}
