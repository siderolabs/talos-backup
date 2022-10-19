package dockertest_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	minio "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	dockertest "github.com/ory/dockertest/v3"
	dc "github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/talos/cmd/talosctl/cmd/mgmt/gen"
	"github.com/talos-systems/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/talos-systems/talos/pkg/machinery/api/machine"
	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/client"
	talosclient "github.com/talos-systems/talos/pkg/machinery/client"
	"github.com/talos-systems/talos/pkg/machinery/client/config"
	talosconfig "github.com/talos-systems/talos/pkg/machinery/client/config"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/resources/v1alpha1"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/siderolabs/talos-backup/cmd/etcd-snapshot-k8s-service/service"
	pkgconfig "github.com/siderolabs/talos-backup/pkg/config"
)

type integrationTestSuite struct {
	suite.Suite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc

	minioResource *dockertest.Resource
	talosResource *dockertest.Resource
	pool          *dockertest.Pool

	serviceConfig pkgconfig.ServiceConfig
	talosConfig   *talosconfig.Config
	talosClient   *talosclient.Client
	minioClient   *minio.Client
}

func TestIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(integrationTestSuite))
}

func (suite *integrationTestSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	var err error

	suite.pool, err = dockertest.NewPool("")
	suite.Require().Nil(err)

	suite.Require().Nil(suite.startMinIO(suite.ctx, suite.pool))

	suite.Require().Nil(suite.startTalosControlPlane(suite.ctx, suite.pool))

	suite.Require().Nil(os.Setenv(awsAccessKeyIDEnvVar, minioRootUser))

	suite.Require().Nil(os.Setenv(awsSecretAccessKeyEnvVar, minioRootPassword))
}

func (suite *integrationTestSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.Require().Nil(cleanup(suite.pool, suite.minioResource, suite.talosResource))

	suite.Require().Nil(os.Unsetenv(awsAccessKeyIDEnvVar))

	suite.Require().Nil(os.Unsetenv(awsSecretAccessKeyEnvVar))
}

const (
	minioRootUser            = "minioadmin"
	minioRootPassword        = "minioadmin"
	awsAccessKeyIDEnvVar     = "AWS_ACCESS_KEY_ID"
	awsSecretAccessKeyEnvVar = "AWS_SECRET_ACCESS_KEY"
)

func applyMachineConfig(cfgBytes []byte) func(ctx context.Context, c *client.Client) error {
	return func(ctx context.Context, c *client.Client) error {
		resp, err := c.ApplyConfiguration(ctx, &machineapi.ApplyConfigurationRequest{
			Data:           cfgBytes,
			Mode:           machine.ApplyConfigurationRequest_AUTO,
			OnReboot:       false,
			Immediate:      true,
			DryRun:         false,
			TryModeTimeout: durationpb.New(constants.ConfigTryTimeout),
		})
		if err != nil {
			return fmt.Errorf("error applying new configuration: %w", err)
		}

		helpers.PrintApplyResults(resp)

		return nil
	}
}

func (suite *integrationTestSuite) startTalosControlPlane(ctx context.Context, pool *dockertest.Pool) error {
	suite.serviceConfig.ClusterName = "talos-test-cluster"

	options := &dockertest.RunOptions{
		Repository: "ghcr.io/siderolabs/talos",
		Cmd:        []string{"server", "/data"},
		PortBindings: map[dc.Port][]dc.PortBinding{
			"50000/tcp": {{HostPort: "50000"}},
			"6443/tcp":  {{HostPort: "6443"}},
		},
		Tag: "v1.3.0-alpha.0-28-gceb0cd99a",
		Env: []string{
			"PLATFORM=container",
		},
		Privileged: true,
		Name:       "talos-test-container-4",
		Hostname:   "talos-test",
		SecurityOpt: []string{
			"seccomp=unconfined",
		},
	}

	hcOpt := func(config *dc.HostConfig) {
		config.ReadonlyRootfs = true
		config.AutoRemove = true
		config.Mounts = []dc.HostMount{
			{
				Type:   "tmpfs",
				Target: "/run",
			},
			{
				Type:   "tmpfs",
				Target: "/system",
			},
			{
				Type:   "tmpfs",
				Target: "/tmp",
			},
			{
				Type:   "volume",
				Target: "/system/state",
			},
			{
				Type:   "volume",
				Target: "/var",
			},
			{
				Type:   "volume",
				Target: "/etc/cni",
			},
			{
				Type:   "volume",
				Target: "/etc/kubernetes",
			},
			{
				Type:   "volume",
				Target: "/usr/libexec/kubernetes",
			},
			{
				Type:   "volume",
				Target: "/usr/etc/udev",
			},
			{
				Type:   "volume",
				Target: "/opt",
			},
		}
	}

	var err error

	suite.talosResource, err = pool.RunWithOptions(options, hcOpt)
	if err != nil {
		return fmt.Errorf("Could not start resource: %w", err)
	}

	endpoint := suite.talosResource.Container.NetworkSettings.IPAddress
	endpointURL := "https://" + endpoint

	bundle, err := gen.V1Alpha1Config(
		nil,
		suite.serviceConfig.ClusterName,
		endpointURL,
		constants.DefaultKubernetesVersion,
		nil,
		nil,
		nil,
	)
	if err != nil {
		return err
	}

	suite.talosConfig = bundle.TalosCfg

	suite.talosClient, err = client.New(ctx, client.WithConfig(suite.talosConfig), client.WithEndpoints(endpoint))
	if err != nil {
		return err
	}

	cfgBytes, err := bundle.ControlPlane().Bytes()
	if err != nil {
		return err
	}

	err = retry(pool, func() error {
		return withMaintenanceClient(ctx, endpoint, applyMachineConfig(cfgBytes))
	})
	if err != nil {
		return err
	}

	err = retry(pool, func() error {
		return withConfigClient(ctx, endpoint, bundle.TalosCfg, func(ctx context.Context, c *client.Client) error {
			return c.Bootstrap(ctx, &machineapi.BootstrapRequest{
				RecoverEtcd:          false,
				RecoverSkipHashCheck: false,
			})
		})
	})
	if err != nil {
		return err
	}

	err = retry(pool, func() error {
		return withConfigClient(ctx, endpoint, bundle.TalosCfg, func(ctx context.Context, c *client.Client) error {
			etcdServiceResource, serviceErr := safe.ReaderGet[*v1alpha1.Service](ctx, c.COSI, v1alpha1.NewService("etcd").Metadata())
			if serviceErr != nil {
				return serviceErr
			}

			if etcdServiceResource.TypedSpec().Running {
				return nil
			}

			return fmt.Errorf("etcd didn't start")
		})
	})
	if err != nil {
		return err
	}

	return nil
}

func withMaintenanceClient(ctx context.Context, endpoint string, action func(context.Context, *client.Client) error) error {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}

	c, err := client.New(ctx, client.WithTLSConfig(tlsConfig), client.WithEndpoints(endpoint))
	if err != nil {
		return err
	}

	//nolint:errcheck
	defer c.Close()

	return action(ctx, c)
}

func withConfigClient(ctx context.Context, endpoint string, cfg *config.Config, action func(context.Context, *client.Client) error) error {
	c, err := client.New(ctx, client.WithConfig(cfg), client.WithEndpoints(endpoint))
	if err != nil {
		return err
	}

	//nolint:errcheck
	defer c.Close()

	return action(ctx, c)
}

func (suite *integrationTestSuite) startMinIO(ctx context.Context, pool *dockertest.Pool) error {
	minioS3APIPort := "9000"

	options := &dockertest.RunOptions{
		Repository: "minio/minio",
		Cmd:        []string{"server", "/data"},
		Tag:        "RELEASE.2022-10-21T22-37-48Z",
		Env: []string{
			"MINIO_ROOT_USER=" + minioRootUser,
			"MINIO_ROOT_PASSWORD=" + minioRootPassword,
		},
	}

	var err error

	suite.minioResource, err = pool.RunWithOptions(options)
	if err != nil {
		return err
	}

	endpoint := suite.minioResource.GetHostPort(minioS3APIPort + "/tcp")
	accessKeyID := minioRootUser
	secretAccessKey := minioRootPassword
	useSSL := false

	suite.minioClient, err = minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return err
	}

	bucketName := "integration-test-bucket"

	err = retry(pool, func() error {
		return suite.minioClient.MakeBucket(ctx, "integration-test-bucket", minio.MakeBucketOptions{
			Region:        "test-region",
			ObjectLocking: false,
		})
	})
	if err != nil {
		return err
	}

	suite.serviceConfig.CustomS3Endpoint = "http://" + suite.minioResource.Container.NetworkSettings.IPAddress + ":" + minioS3APIPort
	suite.serviceConfig.Bucket = bucketName
	suite.serviceConfig.S3Prefix = "testdata/snapshots"
	suite.serviceConfig.AgeX25519PublicKey = "age1khpnnl86pzx96ttyjmldptsl5yn2v9jgmmzcjcufvk00ttkph9zs0ytgec"

	return nil
}

func retry(pool *dockertest.Pool, f func() error) error {
	var lastErr error

	captureLastError := func() error {
		lastErr = f()

		return lastErr
	}

	if err := pool.Retry(captureLastError); err != nil {
		return fmt.Errorf("Could not complete action: %w; last error: %s", err, lastErr)
	}

	return nil
}

func cleanup(pool *dockertest.Pool, resources ...*dockertest.Resource) error {
	for _, resource := range resources {
		if resource != nil {
			if err := pool.Purge(resource); err != nil {
				return fmt.Errorf("Could not purge resource: %w", err)
			}
		}
	}

	return nil
}

func (suite *integrationTestSuite) TestBackupEncryptedSnapshot() {
	// when
	suite.Require().Nil(
		service.BackupEncryptedSnapshot(suite.ctx, &suite.serviceConfig, suite.talosConfig, suite.talosClient),
	)

	// then
	listObjectsChan := suite.minioClient.ListObjects(suite.ctx, suite.serviceConfig.Bucket, minio.ListObjectsOptions{
		// Prefix: suite.serviceConfig.S3Prefix,
		Recursive: true,
	})

	for msg := range listObjectsChan {
		suite.Require().Nil(msg.Err)

		suite.Require().Regexp(regexp.MustCompile(`testdata/snapshots/talos-test-cluster-\d\d\d\d-\d\d-\d\dT\d\d:\d\d:\d\d\+\d\d:\d\d\.snap\.age`), msg.Key)

		suite.Require().Greater(msg.Size, int64(0))
	}
}
