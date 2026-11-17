import { performSignOut, type SignOutSteps } from '../signOut';

function fakeSteps(overrides: Partial<SignOutSteps> = {}) {
  const calls: string[] = [];
  const steps: SignOutSteps = {
    getDeviceId: async () => 'device-1',
    unregisterDevice: async (id) => {
      calls.push(`unregister:${id}`);
    },
    signOut: async () => {
      calls.push('signOut');
    },
    clearCache: () => {
      calls.push('clearCache');
    },
    clearStorage: async () => {
      calls.push('clearStorage');
    },
    ...overrides,
  };
  return { steps, calls };
}

describe('performSignOut', () => {
  it('端末を通知の送信先から外してから（ID トークンが要るため）、サインアウトし、端末上の情報を消す', async () => {
    const { steps, calls } = fakeSteps();

    await performSignOut(steps);

    expect(calls).toEqual(['unregister:device-1', 'signOut', 'clearCache', 'clearStorage']);
  });

  it('端末を登録していなければ、外さずにサインアウトする', async () => {
    const { steps, calls } = fakeSteps({ getDeviceId: async () => null });

    await performSignOut(steps);

    expect(calls).toEqual(['signOut', 'clearCache', 'clearStorage']);
  });

  it('端末を外せなくても（通信できないなど）、サインアウトは止めない', async () => {
    const { steps, calls } = fakeSteps({
      unregisterDevice: async () => {
        throw new Error('network');
      },
    });

    await performSignOut(steps);

    expect(calls).toEqual(['signOut', 'clearCache', 'clearStorage']);
  });
});
