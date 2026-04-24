import { Link } from '@tanstack/react-router'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { AuthLayout } from './auth-layout'

export function ContactAdminNotice({ title }: { title: string }) {
  return (
    <AuthLayout>
      <Card className='max-w-sm gap-4'>
        <CardHeader>
          <CardTitle className='text-lg tracking-tight'>{title}</CardTitle>
          <CardDescription>
            本系统不支持自助注册或找回密码。如需开通账号或重置密码，请联系系统管理员。
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Button asChild variant='outline' className='w-full'>
            <Link to='/sign-in'>返回登录</Link>
          </Button>
        </CardContent>
      </Card>
    </AuthLayout>
  )
}
