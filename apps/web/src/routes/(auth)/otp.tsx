import { createFileRoute } from '@tanstack/react-router'
import { ContactAdminNotice } from '@/features/auth/contact-admin-notice'

export const Route = createFileRoute('/(auth)/otp')({
  component: () => <ContactAdminNotice title='暂不支持验证码登录' />,
})
