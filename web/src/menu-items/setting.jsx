import { Icon } from '@iconify/react';

const icons = {
  IconSitemap: () => <Icon width={20} icon="ph:open-ai-logo-duotone" />,
  IconBasket: () => <Icon width={20} icon="solar:shop-bold-duotone" />,
  IconKey: () => <Icon width={20} icon="solar:key-bold-duotone" />,
  IconUser: () => <Icon width={20} icon="solar:user-bold-duotone" />,
  IconUserScan: () => <Icon width={20} icon="solar:user-id-bold-duotone" />,
  IconReceipt2: () => <Icon width={20} icon="solar:document-bold-duotone" />,
  IconSettingsCog: () => <Icon width={20} icon="solar:settings-bold-duotone" />,
  IconBrandTelegram: () => <Icon width={20} icon="solar:plain-bold-duotone" />,
  IconCoin: () => <Icon width={20} icon="solar:dollar-minimalistic-bold-duotone" />,
  IconBrandPaypal: () => <Icon width={20} icon="solar:wallet-money-bold-duotone" />,
  IconCoins: () => <Icon width={20} icon="solar:hand-money-bold-duotone" />,
  IconUsers: () => <Icon width={20} icon="solar:users-group-rounded-bold-duotone" />,
  IconUsersGroup: () => <Icon width={20} icon="solar:users-group-two-rounded-bold-duotone" />,
  IconUsersPlus: () => <Icon width={20} icon="solar:users-group-two-rounded-bold-duotone" />,
  IconModel: () => <Icon width={20} icon="mingcute:ai-fill" />,
  IconInfo: () => <Icon width={20} icon="solar:info-circle-bold-duotone" />
};

const Setting = {
  id: 'setting',
  title: 'setting',
  type: 'group',
  children: [
    {
      id: 'user',
      title: 'user',
      type: 'item',
      url: '/panel/user',
      icon: icons.IconUser,
      breadcrumbs: false,
      isAdmin: true
    },
    {
      id: 'channel',
      title: 'channel',
      type: 'item',
      url: '/panel/channel',
      icon: icons.IconSitemap,
      breadcrumbs: false,
      isAdmin: true
    },
    {
      id: 'operation',
      title: 'operation',
      type: 'collapse',
      icon: icons.IconBasket,
      isAdmin: true,
      children: [
        {
          id: 'user_group',
          title: 'user_group',
          type: 'item',
          url: '/panel/user_group',
          icon: icons.IconUsers,
          breadcrumbs: false,
          isAdmin: true
        },
        {
          id: 'pricing',
          title: 'pricing',
          type: 'item',
          url: '/panel/pricing',
          icon: icons.IconReceipt2,
          breadcrumbs: false,
          isAdmin: true
        },
        {
          id: 'model_ownedby',
          title: 'modelOwnedby.title',
          type: 'item',
          url: '/panel/model_ownedby',
          icon: icons.IconModel,
          breadcrumbs: false,
          isAdmin: true
        },
        {
          id: 'model_info',
          title: 'modelInfo.modelInfo',
          type: 'item',
          url: '/panel/model_info',
          icon: icons.IconInfo,
          breadcrumbs: false,
          isAdmin: true
        },
        {
          id: 'telegram',
          title: 'Telegram Bot',
          type: 'item',
          url: '/panel/telegram',
          icon: icons.IconBrandTelegram,
          breadcrumbs: false,
          isAdmin: true
        }
      ]
    },
    {
      id: 'paySetting',
      title: 'paySetting',
      type: 'collapse',
      icon: icons.IconBrandPaypal,
      isAdmin: true,
      children: [
        {
          id: 'redemption',
          title: 'redemption',
          type: 'item',
          url: '/panel/redemption',
          icon: icons.IconCoin,
          breadcrumbs: false,
          isAdmin: true
        },
        {
          id: 'payment',
          title: 'payment',
          type: 'item',
          url: '/panel/payment',
          icon: icons.IconBrandPaypal,
          breadcrumbs: false,
          isAdmin: true
        }
      ]
    },

    {
      id: 'token',
      title: 'token',
      type: 'item',
      url: '/panel/token',
      icon: icons.IconKey,
      breadcrumbs: false
    },
    {
      id: 'private_group',
      title: 'private_group',
      type: 'item',
      url: '/panel/private-groups',
      icon: icons.IconUsersGroup,
      breadcrumbs: false
    },

    {
      id: 'profile',
      title: 'profile',
      type: 'item',
      url: '/panel/profile',
      icon: icons.IconUserScan,
      breadcrumbs: false,
      isAdmin: false
    },

    {
      id: 'setting',
      title: 'setting',
      type: 'item',
      url: '/panel/setting',
      icon: icons.IconSettingsCog,
      breadcrumbs: false,
      isAdmin: true
    }
  ]
};

export default Setting;
