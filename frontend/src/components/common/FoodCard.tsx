import { Card, Space, Typography } from 'antd';
import type { FoodItem } from '../../types';
import { DefaultOpenedAfterDays, FoodCategoryLabels, StorageLocationLabels } from '../../constants/food';
import { formatDate } from '../../utils/dateFormat';
import FreshnessBadge from './FreshnessBadge';
import RemainingDaysBar from './RemainingDaysBar';

const { Text } = Typography;

interface Props {
  item: FoodItem;
  onClick?: (item: FoodItem) => void;
}

// 食品卡片：食品管理/统计/推荐共用
export default function FoodCard({ item, onClick }: Props) {
  return (
    <Card
      size="small"
      hoverable={!!onClick}
      onClick={() => onClick?.(item)}
      title={<Space>{item.name}<FreshnessBadge status={item.status} expiryDate={item.expiry_date} /></Space>}
    >
      <Space direction="vertical" size={2} style={{ width: '100%' }}>
        <Text type="secondary">类别：{FoodCategoryLabels[item.category] ?? item.category}</Text>
        <Text type="secondary">数量：{item.quantity} {item.unit}</Text>
        <Text type="secondary">存放：{StorageLocationLabels[item.storage_location] ?? item.storage_location}</Text>
        <Text type="secondary">
          开封：{item.opened_at
            ? `${formatDate(item.opened_at)}（${item.opened_after_days ?? DefaultOpenedAfterDays} 天内食用）`
            : '未开封'}
        </Text>
        <RemainingDaysBar expiryDate={item.expiry_date} />
      </Space>
    </Card>
  );
}
