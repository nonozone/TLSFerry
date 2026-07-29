package deployment

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/nonozone/TLSFerry/internal/certstore"
	"github.com/nonozone/TLSFerry/internal/config"
	"github.com/nonozone/TLSFerry/internal/credential"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	ssl "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/ssl/v20191205"
)

type tencentSSLClient interface {
	UploadCertificateWithContext(context.Context, *ssl.UploadCertificateRequest) (*ssl.UploadCertificateResponse, error)
	DeployCertificateInstanceWithContext(context.Context, *ssl.DeployCertificateInstanceRequest) (*ssl.DeployCertificateInstanceResponse, error)
}

type tencentProvider struct {
	credentials credential.EnvResolver
	newClient   func(string, string, string) (tencentSSLClient, error)
}

func (p tencentProvider) Deploy(ctx context.Context, certificateName string, deployment config.Deployment, bundle certstore.Bundle) (Result, error) {
	values, err := p.credentials.Values(deployment.Credential, "SECRET_ID", "SECRET_KEY")
	if err != nil {
		return Result{}, err
	}
	resourceType, instance, region, err := tencentTarget(deployment)
	if err != nil {
		return Result{}, err
	}
	newClient := p.newClient
	if newClient == nil {
		newClient = func(secretID, secretKey, region string) (tencentSSLClient, error) {
			return ssl.NewClient(common.NewCredential(secretID, secretKey), region, profile.NewClientProfile())
		}
	}
	client, err := newClient(values["SECRET_ID"], values["SECRET_KEY"], region)
	if err != nil {
		return Result{}, fmt.Errorf("create Tencent Cloud SSL client: %w", err)
	}

	repeatable := false
	uploadRequest := ssl.NewUploadCertificateRequest()
	uploadRequest.CertificatePublicKey = stringValuePointer(string(bundle.FullChain()))
	uploadRequest.CertificatePrivateKey = stringValuePointer(string(bundle.PrivateKey))
	uploadRequest.CertificateType = stringValuePointer("SVR")
	uploadRequest.Alias = stringValuePointer("TLSFerry-" + certificateName)
	uploadRequest.Repeatable = &repeatable
	uploadResponse, err := client.UploadCertificateWithContext(ctx, uploadRequest)
	if err != nil {
		return Result{}, fmt.Errorf("upload certificate to Tencent Cloud SSL: %w", err)
	}
	certificateID := ""
	if uploadResponse != nil && uploadResponse.Response != nil {
		if uploadResponse.Response.CertificateId != nil {
			certificateID = *uploadResponse.Response.CertificateId
		} else if uploadResponse.Response.RepeatCertId != nil {
			certificateID = *uploadResponse.Response.RepeatCertId
		}
	}
	if certificateID == "" {
		return Result{}, errors.New("Tencent Cloud SSL upload returned no certificate ID")
	}

	deployRequest := ssl.NewDeployCertificateInstanceRequest()
	deployRequest.CertificateId = &certificateID
	deployRequest.ResourceType = &resourceType
	deployRequest.InstanceIdList = []*string{&instance}
	deployResponse, err := client.DeployCertificateInstanceWithContext(ctx, deployRequest)
	if err != nil {
		return Result{}, fmt.Errorf("submit Tencent Cloud %s deployment: %w", resourceType, err)
	}
	if deployResponse == nil || deployResponse.Response == nil || deployResponse.Response.DeployRecordId == nil {
		return Result{}, errors.New("Tencent Cloud deployment returned no record ID")
	}
	return Result{
		Provider: deployment.Provider, Target: deployment.Target,
		Reference: strconv.FormatUint(*deployResponse.Response.DeployRecordId, 10), Status: "submitted",
	}, nil
}

func tencentTarget(deployment config.Deployment) (resourceType, instance, region string, err error) {
	if err := safeTarget(deployment.Target); err != nil {
		return "", "", "", err
	}
	switch deployment.Provider {
	case "tencent-cdn":
		billing := deployment.Options["billing"]
		if billing == "" {
			billing = "on"
		}
		if billing != "on" && billing != "off" {
			return "", "", "", errors.New("tencent-cdn option billing must be on or off")
		}
		return "cdn", deployment.Target + "|" + billing, "", nil
	case "tencent-cos":
		region = strings.TrimSpace(deployment.Options["region"])
		bucket := strings.TrimSpace(deployment.Options["bucket"])
		if region == "" || bucket == "" || strings.ContainsAny(region+bucket, "|") {
			return "", "", "", errors.New("tencent-cos requires safe region and bucket options")
		}
		return "cos", region + "|" + bucket + "|" + deployment.Target, region, nil
	default:
		return "", "", "", fmt.Errorf("unsupported Tencent Cloud provider %q", deployment.Provider)
	}
}

func safeTarget(target string) error {
	if strings.TrimSpace(target) == "" || strings.ContainsAny(target, "|/\\?#") {
		return fmt.Errorf("unsafe deployment target %q", target)
	}
	return nil
}

func stringValuePointer(value string) *string { return &value }
