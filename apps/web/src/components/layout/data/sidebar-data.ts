import {
  BookOpen,
  Building2,
  Command,
  HelpCircle,
  LayoutDashboard,
  Settings,
} from 'lucide-react'
import type { AuthUser } from '@/stores/auth-store'
import { type NavGroup, type SidebarData } from '../types'

function navGroupsFor(role: AuthUser['role']): NavGroup[] {
  const base: NavGroup[] = [
    {
      title: '主导航',
      items: [
        {
          title: '仪表盘',
          url: '/',
          icon: LayoutDashboard,
        },
        {
          title: '客户',
          url: '/customers',
          icon: Building2,
        },
      ],
    },
  ]

  if (role === 'admin') {
    base.push({
      title: '字典',
      items: [
        {
          title: '国家',
          url: '/catalog/countries',
          icon: BookOpen,
        },
        {
          title: '尼斯分类',
          url: '/catalog/nice-categories',
          icon: BookOpen,
        },
      ],
    })
  }

  base.push({
    title: '系统',
    items: [
      {
        title: '个人设置',
        url: '/settings',
        icon: Settings,
      },
      {
        title: '帮助',
        url: '/help-center',
        icon: HelpCircle,
      },
    ],
  })

  return base
}

export function buildSidebarData(user: AuthUser | null): SidebarData {
  return {
    user: user
      ? { name: user.name, email: user.email, avatar: '/avatars/01.png' }
      : { name: '—', email: '—', avatar: '/avatars/01.png' },
    teams: [
      {
        name: '商标报价平台',
        logo: Command,
        plan: '国际业务',
      },
    ],
    navGroups: user ? navGroupsFor(user.role) : [],
  }
}
