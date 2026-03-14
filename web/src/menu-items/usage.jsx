import { Icon } from '@iconify/react';

const icons = {
  IconArticle: () => <Icon width={20} icon="solar:document-text-bold-duotone" />,
  IconBrush: () => <Icon width={20} icon="tabler:photo-ai" />,
  IconList: () => <Icon width={20} icon="solar:checklist-minimalistic-bold-duotone" />,
  IconInvoice: () => <Icon width={20} icon="solar:dollar-minimalistic-bold-duotone" />
};

const usage = {
  id: 'usage',
  title: 'usage',
  type: 'group',
  children: [
    {
      id: 'log',
      title: 'log',
      type: 'item',
      url: '/panel/log',
      icon: icons.IconArticle,
      breadcrumbs: false
    },
    {
      id: 'invoice',
      title: 'invoice',
      type: 'item',
      url: '/panel/invoice',
      icon: icons.IconInvoice,
      breadcrumbs: false
    },
    {
      id: 'midjourney',
      title: 'midjourney',
      type: 'item',
      url: '/panel/midjourney',
      icon: icons.IconBrush,
      breadcrumbs: false
    },
    {
      id: 'task',
      title: 'task',
      type: 'item',
      url: '/panel/task',
      icon: icons.IconList,
      breadcrumbs: false
    }
  ]
};

export default usage;
