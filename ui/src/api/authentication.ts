import {
  LoginRequest,
  loginResponse,
  LoginResponse,
  UpdateProfileRequest,
  VerifyOtpRequest,
} from '@/schema/authentication.ts'
import axiosInstance from '@/api/axios-instance.ts'

export const login = async (login: LoginRequest): Promise<LoginResponse> => {
  const { data } = await axiosInstance.post('/auth/login', login)
  const parsed = loginResponse.parse(data)
  return parsed.data
}

export const verifyOtp = async (
  req: VerifyOtpRequest
): Promise<LoginResponse> => {
  const { data } = await axiosInstance.post('/auth/otp/verify', req)
  const parsed = loginResponse.parse(data)
  return parsed.data
}

export const updateProfile = async (
  profile: UpdateProfileRequest
): Promise<void> => {
  await axiosInstance.put('/auth/profile', profile)
}
