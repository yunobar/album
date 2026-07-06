package dto

type NewUserRequest struct {
	Email     string
	Password  string
	Name      string
	Avatar    string
	VerifyNow bool
}

type UserResponse struct {
	BaseDTO
	Email   string
	Profile ProfileResponse
}
