import { useSearch } from '@tanstack/react-router'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { AuthLayout } from '../auth-layout'
import { UserAuthForm } from './components/user-auth-form'

export function SignIn() {
  // strict:false lets this component render outside the `(auth)/sign-in` file
  // route (e.g. under the integration test's ad-hoc router) without TanStack
  // Router throwing "route id not found".
  const search = useSearch({ strict: false }) as { redirect?: string }

  return (
    <AuthLayout>
      <Card className='w-full gap-4'>
        <CardHeader>
          <CardTitle className='text-lg tracking-tight'>登录</CardTitle>
          <CardDescription>
            输入账号邮箱与密码登录系统。如需开通账号，请联系系统管理员。
          </CardDescription>
        </CardHeader>
        <CardContent>
          <UserAuthForm redirectTo={search.redirect} />
        </CardContent>
      </Card>
    </AuthLayout>
  )
}
