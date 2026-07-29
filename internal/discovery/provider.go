package discovery

import (
	"fmt"
	"strings"

	"github.com/nonozone/TLSFerry/internal/credential"
)

func NewScanner(provider string, credentials credential.Resolver, reference string) (Scanner, error) {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "tencent":
		return &tencentScanner{credentials: credentials, reference: reference}, nil
	case "aliyun":
		return &aliyunScanner{credentials: credentials, reference: reference}, nil
	case "qiniu":
		return &qiniuScanner{credentials: credentials, reference: reference}, nil
	default:
		return nil, fmt.Errorf("unsupported discovery provider %q", provider)
	}
}
