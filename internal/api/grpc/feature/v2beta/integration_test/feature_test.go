//go:build integration

package feature_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/muhlemmer/gu"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/zitadel/zitadel/internal/integration"
	feature "github.com/zitadel/zitadel/pkg/grpc/feature/v2beta"
	object "github.com/zitadel/zitadel/pkg/grpc/object/v2beta"
)

var (
	IamCTX   context.Context
	OrgCTX   context.Context
	Instance *integration.Instance
	Client   feature.FeatureServiceClient
)

func TestMain(m *testing.M) {
	os.Exit(func() int {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()

		Instance = integration.NewInstance(ctx)
		Client = Instance.Client.FeatureV2beta

		IamCTX = Instance.WithAuthorization(ctx, integration.UserTypeIAMOwner)
		OrgCTX = Instance.WithAuthorization(ctx, integration.UserTypeOrgOwner)

		return m.Run()
	}())
}

/*
System feature tests are removed in this package,
so it does not interfere with the feature/v2 API tests
*/

func TestServer_SetInstanceFeatures(t *testing.T) {
	type args struct {
		ctx context.Context
		req *feature.SetInstanceFeaturesRequest
	}
	tests := []struct {
		name    string
		args    args
		want    *feature.SetInstanceFeaturesResponse
		wantErr bool
	}{
		{
			name: "permission error",
			args: args{
				ctx: OrgCTX,
				req: &feature.SetInstanceFeaturesRequest{
					LoginDefaultOrg: gu.Ptr(true),
				},
			},
			wantErr: true,
		},
		{
			name: "no changes error",
			args: args{
				ctx: IamCTX,
				req: &feature.SetInstanceFeaturesRequest{},
			},
			wantErr: true,
		},
		{
			name: "success",
			args: args{
				ctx: IamCTX,
				req: &feature.SetInstanceFeaturesRequest{
					LoginDefaultOrg: gu.Ptr(true),
				},
			},
			want: &feature.SetInstanceFeaturesResponse{
				Details: &object.Details{
					ChangeDate:    timestamppb.Now(),
					ResourceOwner: Instance.ID(),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {
				// make sure we have a clean state after each test
				_, err := Client.ResetInstanceFeatures(IamCTX, &feature.ResetInstanceFeaturesRequest{})
				require.NoError(t, err)
			})
			got, err := Client.SetInstanceFeatures(tt.args.ctx, tt.args.req)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			integration.AssertDetails(t, tt.want, got)
		})
	}
}

func TestServer_ResetInstanceFeatures(t *testing.T) {
	_, err := Client.SetInstanceFeatures(IamCTX, &feature.SetInstanceFeaturesRequest{
		LoginDefaultOrg: gu.Ptr(true),
	})
	require.NoError(t, err)

	tests := []struct {
		name    string
		ctx     context.Context
		want    *feature.ResetInstanceFeaturesResponse
		wantErr bool
	}{
		{
			name:    "permission error",
			ctx:     OrgCTX,
			wantErr: true,
		},
		{
			name: "success",
			ctx:  IamCTX,
			want: &feature.ResetInstanceFeaturesResponse{
				Details: &object.Details{
					ChangeDate:    timestamppb.Now(),
					ResourceOwner: Instance.ID(),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Client.ResetInstanceFeatures(tt.ctx, &feature.ResetInstanceFeaturesRequest{})
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			integration.AssertDetails(t, tt.want, got)
		})
	}
}

func TestServer_GetInstanceFeatures(t *testing.T) {
	type args struct {
		ctx context.Context
		req *feature.GetInstanceFeaturesRequest
	}
	tests := []struct {
		name    string
		prepare func(t *testing.T)
		args    args
		want    *feature.GetInstanceFeaturesResponse
		wantErr bool
	}{
		{
			name: "permission error",
			args: args{
				ctx: OrgCTX,
				req: &feature.GetInstanceFeaturesRequest{},
			},
			wantErr: true,
		},
		{
			name: "defaults, no inheritance",
			args: args{
				ctx: IamCTX,
				req: &feature.GetInstanceFeaturesRequest{},
			},
			want: &feature.GetInstanceFeaturesResponse{},
		},
		{
			name: "defaults, inheritance",
			args: args{
				ctx: IamCTX,
				req: &feature.GetInstanceFeaturesRequest{
					Inheritance: true,
				},
			},
			want: &feature.GetInstanceFeaturesResponse{
				LoginDefaultOrg: &feature.FeatureFlag{
					Enabled: false,
					Source:  feature.Source_SOURCE_UNSPECIFIED,
				},
				UserSchema: &feature.FeatureFlag{
					Enabled: false,
					Source:  feature.Source_SOURCE_UNSPECIFIED,
				},
			},
		},
		{
			name: "some features, no inheritance",
			prepare: func(t *testing.T) {
				_, err := Client.SetInstanceFeatures(IamCTX, &feature.SetInstanceFeaturesRequest{
					LoginDefaultOrg: gu.Ptr(true),
					UserSchema:      gu.Ptr(true),
				})
				require.NoError(t, err)
			},
			args: args{
				ctx: IamCTX,
				req: &feature.GetInstanceFeaturesRequest{},
			},
			want: &feature.GetInstanceFeaturesResponse{
				LoginDefaultOrg: &feature.FeatureFlag{
					Enabled: true,
					Source:  feature.Source_SOURCE_INSTANCE,
				},
				UserSchema: &feature.FeatureFlag{
					Enabled: true,
					Source:  feature.Source_SOURCE_INSTANCE,
				},
			},
		},
		{
			name: "one feature, inheritance",
			prepare: func(t *testing.T) {
				_, err := Client.SetInstanceFeatures(IamCTX, &feature.SetInstanceFeaturesRequest{
					LoginDefaultOrg: gu.Ptr(true),
				})
				require.NoError(t, err)
			},
			args: args{
				ctx: IamCTX,
				req: &feature.GetInstanceFeaturesRequest{
					Inheritance: true,
				},
			},
			want: &feature.GetInstanceFeaturesResponse{
				LoginDefaultOrg: &feature.FeatureFlag{
					Enabled: true,
					Source:  feature.Source_SOURCE_INSTANCE,
				},
				UserSchema: &feature.FeatureFlag{
					Enabled: false,
					Source:  feature.Source_SOURCE_UNSPECIFIED,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {
				// make sure we have a clean state after each test
				_, err := Client.ResetInstanceFeatures(IamCTX, &feature.ResetInstanceFeaturesRequest{})
				require.NoError(t, err)
			})
			if tt.prepare != nil {
				tt.prepare(t)
			}
			got, err := Client.GetInstanceFeatures(tt.args.ctx, tt.args.req)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assertFeatureFlag(t, tt.want.LoginDefaultOrg, got.LoginDefaultOrg)
			assertFeatureFlag(t, tt.want.UserSchema, got.UserSchema)
		})
	}
}

func assertFeatureFlag(t *testing.T, expected, actual *feature.FeatureFlag) {
	t.Helper()
	assert.Equal(t, expected.GetEnabled(), actual.GetEnabled(), "enabled")
	assert.Equal(t, expected.GetSource(), actual.GetSource(), "source")
}
