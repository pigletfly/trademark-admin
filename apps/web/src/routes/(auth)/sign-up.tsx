import { createFileRoute } from '@tanstack/react-router'
import { ContactAdminNotice } from '@/features/auth/contact-admin-notice'

export const Route = createFileRoute('/(auth)/sign-up')({
  component: () => <ContactAdminNotice title='无法自助注册' />,
})
