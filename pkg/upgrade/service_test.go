package upgrade

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/Jguer/aur"
	"github.com/Jguer/aur/metadata"
	"github.com/Jguer/go-alpm/v2"
	"github.com/stretchr/testify/assert"

	"github.com/Jguer/yay/v11/pkg/db"
	"github.com/Jguer/yay/v11/pkg/db/mock"
	"github.com/Jguer/yay/v11/pkg/dep"
	"github.com/Jguer/yay/v11/pkg/settings"
	"github.com/Jguer/yay/v11/pkg/settings/parser"
	"github.com/Jguer/yay/v11/pkg/text"
	"github.com/Jguer/yay/v11/pkg/topo"
	"github.com/Jguer/yay/v11/pkg/vcs"

	mockaur "github.com/Jguer/yay/v11/pkg/dep/mock"
)

func ptrString(s string) *string {
	return &s
}

func TestUpgradeService_GraphUpgrades(t *testing.T) {
	linuxDepInfo := &dep.InstallInfo{
		Reason:       dep.Explicit,
		Source:       dep.Sync,
		AURBase:      nil,
		LocalVersion: "4.5.0-1",
		Version:      "5.0.0-1",
		SyncDBName:   ptrString("core"),
		Upgrade:      true,
		Devel:        false,
	}

	exampleDepInfoDevel := &dep.InstallInfo{
		Source:       dep.AUR,
		Reason:       dep.Dep,
		AURBase:      ptrString("example"),
		LocalVersion: "2.2.1.r32.41baa362-1",
		Version:      "",
		Upgrade:      true,
		Devel:        true,
	}

	exampleDepInfoAUR := &dep.InstallInfo{
		Source:       dep.AUR,
		Reason:       dep.Dep,
		AURBase:      ptrString("example"),
		LocalVersion: "2.2.1.r32.41baa362-1",
		Version:      "2.2.1.r69.g8a10460-1",
		Upgrade:      true,
		Devel:        false,
	}

	yayDepInfo := &dep.InstallInfo{
		Reason:       dep.Explicit,
		Source:       dep.AUR,
		AURBase:      ptrString("yay"),
		LocalVersion: "10.2.3",
		Version:      "10.2.4",
		Upgrade:      true,
		Devel:        false,
	}

	dbExe := &mock.DBExecutor{
		InstalledRemotePackageNamesFn: func() []string {
			return []string{"yay", "example-git"}
		},
		InstalledRemotePackagesFn: func() map[string]mock.IPackage {
			mapRemote := make(map[string]mock.IPackage)
			mapRemote["yay"] = &mock.Package{
				PName:    "yay",
				PBase:    "yay",
				PVersion: "10.2.3",
				PReason:  alpm.PkgReasonExplicit,
			}

			mapRemote["example-git"] = &mock.Package{
				PName:    "example-git",
				PBase:    "example",
				PVersion: "2.2.1.r32.41baa362-1",
				PReason:  alpm.PkgReasonDepend,
			}

			return mapRemote
		},
		SyncUpgradesFn: func(bool) (map[string]db.SyncUpgrade, error) {
			mapUpgrades := make(map[string]db.SyncUpgrade)

			coreDB := mock.NewDB("core")
			mapUpgrades["linux"] = db.SyncUpgrade{
				Package: &mock.Package{
					PName:    "linux",
					PVersion: "5.0.0-1",
					PReason:  alpm.PkgReasonDepend,
					PDB:      coreDB,
				},
				LocalVersion: "4.5.0-1",
				Reason:       alpm.PkgReasonExplicit,
			}
			return mapUpgrades, nil
		},
		ReposFn: func() []string { return []string{"core"} },
	}
	vcsStore := &vcs.Mock{
		ToUpgradeReturn: []string{"example-git"},
	}

	mockAUR := &mockaur.MockAUR{
		GetFn: func(ctx context.Context, query *metadata.AURQuery) ([]aur.Pkg, error) {
			return []aur.Pkg{
				{Name: "yay", Version: "10.2.4", PackageBase: "yay"},
				{Name: "example-git", Version: "2.2.1.r69.g8a10460-1", PackageBase: "example"},
			}, nil
		},
	}
	grapher := dep.NewGrapher(dbExe, mockAUR,
		false, true, io.Discard, false, false)

	cfg := &settings.Configuration{
		Runtime: &settings.Runtime{Mode: parser.ModeAny},
	}

	type fields struct {
		input     io.Reader
		output    io.Writer
		noConfirm bool
		devel     bool
	}
	type args struct {
		graph           *topo.Graph[string, *dep.InstallInfo]
		enableDowngrade bool
	}
	tests := []struct {
		name         string
		fields       fields
		args         args
		mustExist    map[string]*dep.InstallInfo
		mustNotExist map[string]bool
		wantErr      bool
	}{
		{
			name: "no input",
			fields: fields{
				input:     strings.NewReader("\n"),
				output:    io.Discard,
				noConfirm: false,
			},
			args: args{
				graph:           nil,
				enableDowngrade: false,
			},
			mustExist: map[string]*dep.InstallInfo{
				"yay":         yayDepInfo,
				"linux":       linuxDepInfo,
				"example-git": exampleDepInfoAUR,
			},
			mustNotExist: map[string]bool{},
			wantErr:      false,
		},
		{
			name: "no input devel",
			fields: fields{
				input:     strings.NewReader("\n"),
				output:    io.Discard,
				noConfirm: false,
				devel:     true,
			},
			args: args{
				graph:           nil,
				enableDowngrade: false,
			},
			mustExist: map[string]*dep.InstallInfo{
				"yay":         yayDepInfo,
				"linux":       linuxDepInfo,
				"example-git": exampleDepInfoDevel,
			},
			mustNotExist: map[string]bool{},
			wantErr:      false,
		},
		{
			name: "exclude yay",
			fields: fields{
				input:     strings.NewReader("1\n"),
				output:    io.Discard,
				noConfirm: false,
			},
			args: args{
				graph:           nil,
				enableDowngrade: false,
			},
			mustExist: map[string]*dep.InstallInfo{
				"linux":       linuxDepInfo,
				"example-git": exampleDepInfoAUR,
			},
			mustNotExist: map[string]bool{"yay": true},
			wantErr:      false,
		},
		{
			name: "exclude linux",
			fields: fields{
				input:     strings.NewReader("3\n"),
				output:    io.Discard,
				noConfirm: false,
			},
			args: args{
				graph:           nil,
				enableDowngrade: false,
			},
			mustExist: map[string]*dep.InstallInfo{
				"yay":         yayDepInfo,
				"example-git": exampleDepInfoAUR,
			},
			mustNotExist: map[string]bool{"linux": true},
			wantErr:      false,
		},
		{
			name: "only linux",
			fields: fields{
				input:     strings.NewReader("^3\n"),
				output:    io.Discard,
				noConfirm: false,
			},
			args: args{
				graph:           nil,
				enableDowngrade: false,
			},
			mustExist: map[string]*dep.InstallInfo{
				"linux": linuxDepInfo,
			},
			mustNotExist: map[string]bool{"yay": true, "example-git": true},
			wantErr:      false,
		},
		{
			name: "exclude all",
			fields: fields{
				input:     strings.NewReader("1-3\n"),
				output:    io.Discard,
				noConfirm: false,
			},
			args: args{
				graph:           nil,
				enableDowngrade: false,
			},
			mustExist:    map[string]*dep.InstallInfo{},
			mustNotExist: map[string]bool{"yay": true, "example-git": true, "linux": true},
			wantErr:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg.Devel = tt.fields.devel
			u := &UpgradeService{
				log:        text.NewLogger(tt.fields.output, tt.fields.input, true, "test"),
				grapher:    grapher,
				aurCache:   mockAUR,
				dbExecutor: dbExe,
				vcsStore:   vcsStore,
				runtime:    cfg.Runtime,
				cfg:        cfg,
				noConfirm:  tt.fields.noConfirm,
			}

			got, err := u.GraphUpgrades(context.Background(), tt.args.graph, tt.args.enableDowngrade)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpgradeService.GraphUpgrades() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			for node, info := range tt.mustExist {
				assert.True(t, got.Exists(node), node)
				assert.Equal(t, info, got.GetNodeInfo(node).Value)
			}

			for node := range tt.mustNotExist {
				assert.False(t, got.Exists(node), node)
			}
		})
	}
}