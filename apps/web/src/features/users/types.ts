export type UserRole = 'salesperson' | 'reviewer' | 'admin'
export type UserStatus = 'active' | 'disabled'

export interface User {
  id: string
  name: string
  email: string
  phone?: string
  role: UserRole
  status: UserStatus
}

export interface CreateUserRequest {
  name: string
  email: string
  phone?: string
  role: UserRole
  password: string
}

export interface UpdateUserRequest {
  name?: string
  phone?: string
  role?: UserRole
  status?: UserStatus
}

export interface UserListQuery {
  q?: string
  role?: UserRole | ''
  page?: number
  page_size?: number
}

export interface UserListResponse {
  items: User[]
  page: number
  page_size: number
  total: number
}

export interface ResetPasswordResponse {
  password: string
}
