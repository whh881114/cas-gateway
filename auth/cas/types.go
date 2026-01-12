package cas

import "encoding/xml"

// ServiceResponse CAS 服务验证响应（XML格式）
type ServiceResponse struct {
	XMLName xml.Name `xml:"serviceResponse"`
	Success *SuccessResponse
	Failure *FailureResponse
}

// SuccessResponse CAS 成功响应（XML格式）
type SuccessResponse struct {
	XMLName    xml.Name    `xml:"authenticationSuccess"`
	User       string      `xml:"user"`
	Attributes *Attributes `xml:"attributes,omitempty"`
}

// FailureResponse CAS 失败响应（XML格式）
type FailureResponse struct {
	XMLName xml.Name `xml:"authenticationFailure"`
	Code    string   `xml:"code,attr"`
	Message string   `xml:",chardata"`
}

// Attributes CAS 用户属性（XML格式）
type Attributes struct {
	Email       string `xml:"email,omitempty"`
	DisplayName string `xml:"displayName,omitempty"`
}

// JSONServiceResponse CAS 服务验证响应（JSON格式）
type JSONServiceResponse struct {
	ServiceResponse JSONServiceResponseInner `json:"serviceResponse"`
}

// JSONServiceResponseInner CAS JSON响应内部结构
type JSONServiceResponseInner struct {
	AuthenticationSuccess *JSONSuccessResponse `json:"authenticationSuccess,omitempty"`
	AuthenticationFailure *JSONFailureResponse `json:"authenticationFailure,omitempty"`
}

// JSONSuccessResponse CAS 成功响应（JSON格式）
type JSONSuccessResponse struct {
	User       string         `json:"user"`
	Attributes JSONAttributes `json:"attributes,omitempty"`
}

// JSONFailureResponse CAS 失败响应（JSON格式）
type JSONFailureResponse struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

// JSONAttributes CAS 用户属性（JSON格式，oaid和employeeName都是数组）
type JSONAttributes struct {
	Oaid         []string `json:"oaid,omitempty"`
	EmployeeName []string `json:"employeeName,omitempty"`
}
