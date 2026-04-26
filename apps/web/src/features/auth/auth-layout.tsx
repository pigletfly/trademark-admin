import { Logo } from '@/assets/logo'

type AuthLayoutProps = {
  children: React.ReactNode
}

export function AuthLayout({ children }: AuthLayoutProps) {
  return (
    <div className='container flex h-svh max-w-none items-center justify-center'>
      <div className='mx-auto flex w-full max-w-md flex-col justify-center space-y-2 py-8 sm:p-8'>
        <div className='mb-4 flex items-center justify-center'>
          <Logo className='me-2' />
          <h1 className='text-xl font-medium'>商标报价平台</h1>
        </div>
        {children}
      </div>
    </div>
  )
}
