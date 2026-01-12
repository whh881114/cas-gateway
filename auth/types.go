package auth

// UserInfo 用户信息
type UserInfo struct {
	Oaid         string                 `json:"oaid"`
	EmployeeName string                 `json:"employeeName"`
	Extra        map[string]interface{} `json:"extra"`
}
