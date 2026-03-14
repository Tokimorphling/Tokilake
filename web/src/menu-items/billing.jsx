import { Icon } from '@iconify/react';

const icons = {
  IconCreditCard: () => <Icon width={20} icon="solar:card-bold-duotone" />,
  IconBrandGithubCopilot: () => <Icon width={20} icon="solar:box-minimalistic-bold-duotone" />
};

const Billing = {
  id: 'billing',
  title: 'billing',
  type: 'group',
  children: [
    {
      id: 'topup',
      title: 'topup',
      type: 'item',
      url: '/panel/topup',
      icon: icons.IconCreditCard,
      breadcrumbs: false
    },
    {
      id: 'model_price',
      title: 'model_price',
      type: 'item',
      url: '/panel/model_price',
      icon: icons.IconBrandGithubCopilot,
      breadcrumbs: false,
      isAdmin: false
    }
  ]
};

export default Billing;
