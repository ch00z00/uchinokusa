import { router } from 'expo-router';
import { useTranslation } from 'react-i18next';

import { ListRow } from '@/components/ListRow';
import { formatDate, formatDay } from '@/lib/formatDate';

import type { Plant } from './plantList';

type Props = {
  plant: Plant;
  /** 置き場所名。未設定、または一覧にない（削除された）ときは undefined */
  locationName: string | undefined;
  /** アーカイブした植物の一覧（PLANT-05）なら true。右に、予定日の代わりにアーカイブした日を出す */
  archived?: boolean;
};

/**
 * 植物の一覧の項目（foundations.md の 5章の PlantListItem）。押すと植物の詳細（PLANT-10）。
 * 次の予定日の作業の種類は、API に足されてから出す（HANDOFF-mobile.md のバックエンドへの依頼3）
 */
export function PlantListItem({ plant, locationName, archived }: Props) {
  const { t } = useTranslation();
  const species = plant.speciesName ?? plant.speciesFreeText;
  return (
    <ListRow
      title={plant.name}
      subtitle={[locationName ?? t('plants.list.locationUnset'), species]
        .filter(Boolean)
        .join(t('plants.list.separator'))}
      detail={
        archived
          ? plant.archivedAt && t('plants.archived.archivedOn', { date: formatDate(plant.archivedAt) })
          : plant.nextDueOn
            ? formatDay(plant.nextDueOn)
            : t('plants.list.noTask')
      }
      navigates
      onPress={() => router.push({ pathname: '/plants/[id]', params: { id: plant.id } })}
    />
  );
}
