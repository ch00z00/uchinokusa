import { gardenShopUrls, openGardenShopSearch } from '../gardenShop';

describe('gardenShopUrls', () => {
  it('iOS は地図アプリ（Apple マップ）、Android は地図アプリ（geo:）で検索し、だめならブラウザの Google マップ', () => {
    expect(gardenShopUrls('ios', '園芸店')).toEqual([
      'maps://?q=%E5%9C%92%E8%8A%B8%E5%BA%97',
      'https://www.google.com/maps/search/?api=1&query=%E5%9C%92%E8%8A%B8%E5%BA%97',
    ]);
    expect(gardenShopUrls('android', '園芸店')[0]).toBe('geo:0,0?q=%E5%9C%92%E8%8A%B8%E5%BA%97');
  });

  it('ほかの環境（ブラウザ）は、Google マップだけ', () => {
    expect(gardenShopUrls('web', '園芸店')).toEqual([
      'https://www.google.com/maps/search/?api=1&query=%E5%9C%92%E8%8A%B8%E5%BA%97',
    ]);
  });
});

describe('openGardenShopSearch', () => {
  it('地図アプリを開けなければ、ブラウザで開く（要件18-2）', async () => {
    const opened: string[] = [];
    const open = jest.fn(async (url: string) => {
      if (url.startsWith('maps:')) throw new Error('no app');
      opened.push(url);
    });
    await expect(openGardenShopSearch('ios', '園芸店', open)).resolves.toBe(true);
    expect(open).toHaveBeenCalledTimes(2);
    expect(opened[0]).toContain('google.com/maps');
  });

  it('どれも開けなければ false', async () => {
    await expect(
      openGardenShopSearch('ios', '園芸店', async () => {
        throw new Error('x');
      }),
    ).resolves.toBe(false);
  });
});
