import { usePaymentStatus } from '@/hooks/use-payment-status';
import { PaymentStatus } from '@/types/payment';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Link } from 'react-router-dom';
import { ClipboardList } from 'lucide-react';
import { statusColor, paymentStatusColor } from './OrderListPage';
import { Order } from '@/types/api';
import React from 'react';

interface OrderRowProps {
  order: Order;
}

export const OrderRow: React.FC<OrderRowProps> = ({ order }) => {
  const { payment, loading: paymentLoading } = usePaymentStatus(order.id, order.status === 'PLACED' || order.status === 'FAILED');
  return (
    <div key={order.id} className="p-4 rounded-xl bg-card shadow-card border border-border flex items-center justify-between">
      <div>
        <div className="flex items-center gap-3 mb-1">
          <span className="font-display font-semibold">Order #{order.id}</span>
          <Badge className={statusColor[order.status] || ''}>{order.status.replace(/_/g, ' ')}</Badge>
          {payment && payment.status && (
            <Badge className={paymentStatusColor[payment.status] + ' ml-2'}>
              {typeof payment.status === 'string' ? payment.status.replace(/_/g, ' ') : ''}
            </Badge>
          )}
        </div>
        <p className="text-sm text-muted-foreground">{order.item_name} — ₹{order.item_price.toFixed(2)}</p>
        <p className="text-xs text-muted-foreground mt-1">{new Date(order.created_at).toLocaleString()}</p>
        {payment && payment.status === 'FAILED' && (
          <div className="text-xs text-destructive mt-1">Payment failed. <Button size="sm" variant="outline" className="ml-2" asChild><Link to={`/orders/${order.id}`}>Retry</Link></Button></div>
        )}
        {payment && payment.status === 'PENDING_PAYMENT' && (
          <div className="text-xs text-warning mt-1">Awaiting payment...</div>
        )}
        {payment && payment.status === 'REFUNDED' && (
          <div className="text-xs text-muted-foreground mt-1">Payment refunded.</div>
        )}
      </div>
      {/* ...other order actions/buttons... */}
    </div>
  );
};
