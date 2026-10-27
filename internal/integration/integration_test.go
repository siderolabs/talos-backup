// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package dockertest_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/cosi-project/runtime/pkg/safe"
	dockertest "github.com/ory/dockertest/v3"
	dc "github.com/ory/dockertest/v3/docker"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/gen"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	talosclient "github.com/siderolabs/talos/pkg/machinery/client"
	talosconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/gendata"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/siderolabs/talos-backup/cmd/talos-backup/service"
	pkgconfig "github.com/siderolabs/talos-backup/pkg/config"
)

type integrationTestSuite struct {
	suite.Suite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc

	s3Resource    *dockertest.Resource
	talosResource *dockertest.Resource
	pool          *dockertest.Pool

	talosConfig *talosconfig.Config
	talosClient *talosclient.Client
	s3Client    *s3.Client

	serviceConfig pkgconfig.ServiceConfig
}

func TestIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(integrationTestSuite))
}

func (suite *integrationTestSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	var err error

	suite.pool, err = dockertest.NewPool("")
	suite.Require().Nil(err)

	err = suite.pool.Client.Ping()
	suite.Require().Nil(err)

	suite.Require().Nil(suite.startS3(suite.ctx, suite.pool))

	suite.Require().Nil(suite.startTalosControlPlane(suite.ctx, suite.pool))

	suite.Require().Nil(os.Setenv("AWS_REGION", "us-east-1"))
	suite.Require().Nil(os.Setenv(awsAccessKeyIDEnvVar, s3AccessKey))

	suite.Require().Nil(os.Setenv(awsSecretAccessKeyEnvVar, s3SecretKey))
}

func (suite *integrationTestSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.Require().Nil(cleanup(suite.pool, suite.s3Resource, suite.talosResource))

	suite.Require().Nil(os.Unsetenv(awsAccessKeyIDEnvVar))

	suite.Require().Nil(os.Unsetenv(awsSecretAccessKeyEnvVar))
}

const (
	s3AccessKey              = "talosbackup"
	s3SecretKey              = "talosbackup"
	awsAccessKeyIDEnvVar     = "AWS_ACCESS_KEY_ID"
	awsSecretAccessKeyEnvVar = "AWS_SECRET_ACCESS_KEY"
)

func applyMachineConfig(cfgBytes []byte) func(ctx context.Context, c *talosclient.Client) error {
	return func(ctx context.Context, c *talosclient.Client) error {
		resp, err := c.ApplyConfiguration(ctx, &machineapi.ApplyConfigurationRequest{
			Data:           cfgBytes,
			Mode:           machineapi.ApplyConfigurationRequest_AUTO,
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
		Tag: gendata.VersionTag,
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

	// NetworkSettings.IPAddress is deprecated and empty on recent Docker versions
	endpoint := suite.talosResource.Container.NetworkSettings.Networks["bridge"].IPAddress
	endpointURL := "https://" + endpoint

	bundle, err := gen.GenerateConfigBundle(
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

	suite.talosClient, err = talosclient.New(ctx, talosclient.WithConfig(suite.talosConfig), talosclient.WithEndpoints(endpoint))
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
		return withConfigClient(ctx, endpoint, bundle.TalosCfg, func(ctx context.Context, c *talosclient.Client) error {
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
		return withConfigClient(ctx, endpoint, bundle.TalosCfg, func(ctx context.Context, c *talosclient.Client) error {
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

func withMaintenanceClient(ctx context.Context, endpoint string, action func(context.Context, *talosclient.Client) error) error {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}

	c, err := talosclient.New(ctx, talosclient.WithTLSConfig(tlsConfig), talosclient.WithEndpoints(endpoint))
	if err != nil {
		return err
	}

	//nolint:errcheck
	defer c.Close()

	return action(ctx, c)
}

func withConfigClient(ctx context.Context, endpoint string, cfg *talosconfig.Config, action func(context.Context, *talosclient.Client) error) error {
	c, err := talosclient.New(ctx, talosclient.WithConfig(cfg), talosclient.WithEndpoints(endpoint))
	if err != nil {
		return err
	}

	//nolint:errcheck
	defer c.Close()

	return action(ctx, c)
}

func (suite *integrationTestSuite) startS3(ctx context.Context, pool *dockertest.Pool) error {
	s3APIPort := "7070"

	options := &dockertest.RunOptions{
		Repository:   "versity/versitygw",
		Cmd:          []string{"--access", s3AccessKey, "--secret", s3SecretKey, "posix", "/tmp"},
		Tag:          "v1.10.0",
		ExposedPorts: []string{s3APIPort + "/tcp"},
	}

	var err error

	suite.s3Resource, err = pool.RunWithOptions(options)
	if err != nil {
		fmt.Printf("failed to run")
		return err
	}

	endpoint := "http://" + suite.s3Resource.GetHostPort(s3APIPort+"/tcp")

	suite.s3Client = s3.New(s3.Options{
		BaseEndpoint: aws.String(endpoint),
		Region:       "us-east-1",
		UsePathStyle: true,
		Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
			return aws.Credentials{AccessKeyID: s3AccessKey, SecretAccessKey: s3SecretKey}, nil
		}),
	})

	bucketName := "integration-test-bucket"

	err = retry(pool, func() error {
		_, createErr := suite.s3Client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucketName)})

		return createErr
	})
	if err != nil {
		return err
	}

	suite.serviceConfig.CustomS3Endpoint = endpoint
	suite.serviceConfig.Bucket = bucketName
	suite.serviceConfig.S3Prefix = "testdata/snapshots"
	suite.serviceConfig.UsePathStyle = true

	return nil
}

func retry(pool *dockertest.Pool, f func() error) error {
	var lastErr error

	captureLastError := func() error {
		lastErr = f()

		return lastErr
	}

	if err := pool.Retry(captureLastError); err != nil {
		return fmt.Errorf("Could not complete action: %w; last error: %w", err, lastErr)
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
	suite.serviceConfig.AgeRecipientPublicKey = []string{
		"age1khpnnl86pzx96ttyjmldptsl5yn2v9jgmmzcjcufvk00ttkph9zs0ytgec",
		"age1majtfe4q0u030xcjg0ent45stsfev28fjwy0saxqw4c3x85gge5s5gtkwx",
	}
	suite.Require().Nil(
		service.BackupSnapshot(suite.ctx, &suite.serviceConfig, suite.talosConfig, suite.talosClient, true, false),
	)
	suite.serviceConfig.AgeRecipientPublicKey = []string{
		"age1pq1sczsw48n4z6tw38el2kfyuw0436j630r37phwyvfjw7kvgnlfe5qstmt5lecwhqk39kwjjcp42jtukw4vtwgjnwrvwy8aajdjxggr0c5hj4x4dfm2vxhu6kylj39tcwjk06cvzw8dp3rk2d44mx85tm8jrd5nxn2wznedz9nc74thrh29ysyenr8krzu02ccy9kgx9wgqa28tdxk8jxga6y46wqv2p0g2079g6zkkdvj87560ppnjw42n4e5ky94rny4ndq5sxx90l8kwqqnp9036ycem4v0wsw8rjtqq7t9szqlzfyquwahzmvqx29xdcqk90hn295w5vg2s9u5zaryesvwzgq55aqvcfzdfuzx0tmpf8v93jkv8x7u2kqwlvfrdhrspt3mxqhkc699mtyy38mfacg52yrpfdw0mfk8k2ajxevjf952540fjpq982xagvtx8nkfuvt4rx8xfrslqvw4zc4gl4gy05cm0thqfgyvgegnkd2twa9ra82ng6n9vpx2g5hrw3gkfdajczwg4djngzn33j9d0zva4jqen3sj3nakxx93cerycksgcr2kwtasj02qrspjy3pltanzqk7gmqr8fcuutwh0xsrcgeetrvw84xem5wrvrf3zdx4hf9sugrm20nhudaw5xxqpts7gf44zrvth2qx5cyzrch7l2w9cx3uuvhk2uzh3x2c2ktfmvw7jk2alk06k078x9tpqestzjjvxzeyxgyt3fhqc8q4axhk5vdvncecu6zyqu4dtne52hd36w4vaasrgf4n4yyfny62kffjfxew78gqr9vrtc4gzezd03vwqumrjm5z5qfqquk5enu9zxttu75m85mztu7r5fxtqnu9xqhtzlj7jgp6guhnnwd9zga8a8vcjgffqs6pfpkvgwjfmj73zkwnhewe2j3rxd63gudl23wg8k9maz8qvdpul9u4f9fm6qvxr49ktkvmxpfjq4p4cskfypc7rks8453t099xrqu6s7ky4k8x2glk4x4234ggmm43yva86qh86z7pe4vtl9e2uzn33lmp9559ykt2v64xwmea2w454mur2yartjdyth223q4vk62ctucaszcy0rrhxap536jydqdxpvtepz689hsmkcea72ff8uacvfj8ne99tkl9d2xcwj4s9uudhpadesmvynt53qfrkkwpphhc8k6xrc35gg6ghjerp6ee7x5zfqexxpx62r0dj2x5dcun26n4f2a5f873jc6je23gm4e6dm8qc26cvqehv5edwjwmrx6ch7gwyvktf04kuxvus4xkvrc4kmt9vhsl956r43z6nxg6m5ffgz2u8pjxj3cg8yljcuufgqeqgfzdhh0jmjug9jhv8x2jakwvx4fghdp0qxp6rfdkcddgznuqm205qrluztst3jet3djth675rqjrrd5u82a3vrj03txdr56d5pk99ym35q54hhxd54drgjpk9ngyk5ww9vzu6vsuky9vkvgg8xcvsqye3dkx7pdsa658fusaqs249yxwfkeh7fg0rljqrnryuv72rm0uhh4f40nyaxydya9xrzyy579tswntaftw95wq7uwjv3xsf3kpqrym3y3cgsa008xyd7zs9qcy34zfdfzscm2jvrxdjsuzr6f5ksqssq9vjpy6de3d6jems9ajetk3dx820awkh0wu8qzv6vcck2gmsyvtc5cw6u6zsxj6k9dggsjq3g854nqe5dg43r4jpj69sdvruk9x562s2s3kyd8c3dkchh2e4pnpywtyp7a6em63r9k8sdkrgyqz9lvgrjy9pw8eua9r8tn0r25mulgl6u38gpr4v0addpwnc2v387vt0yd28as2jcynl2d60wkze4c9mq5h54utsha6fyz5s7zq36dfsy5xg5yv8wdjdhtu4wv278vunhx85qqa7cc8f",
	}
	suite.Require().Nil(
		service.BackupSnapshot(suite.ctx, &suite.serviceConfig, suite.talosConfig, suite.talosClient, true, false),
	)

	// then
	objects, err := suite.s3Client.ListObjectsV2(suite.ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(suite.serviceConfig.Bucket),
	})
	suite.Require().Nil(err)
	suite.Require().NotEmpty(objects.Contents)

	for _, obj := range objects.Contents {
		suite.Require().Regexp(regexp.MustCompile(`testdata/snapshots/talos-test-cluster-\d\d\d\d-\d\d-\d\dT\d\d:\d\d:\d\dZ\.snap\.zst\.age`), aws.ToString(obj.Key))

		suite.Require().Greater(aws.ToInt64(obj.Size), int64(0))
	}
}
