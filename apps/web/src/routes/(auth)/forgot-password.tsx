import { createFileRoute } from "@tanstack/react-router"
import { ContactAdminNotice } from "@/features/auth/contact-admin-notice"

export const Route = createFileRoute("/(auth)/forgot-password")({
  component: () => <ContactAdminNotice title="无法自助找回密码" />,
})
