package main

import (
	"context"
	"google.golang.org/grpc"
	"github.com/bishopfox/sliver/protobuf/clientpb"
	"github.com/bishopfox/sliver/protobuf/commonpb"
	"github.com/bishopfox/sliver/protobuf/sliverpb"
	"github.com/bishopfox/sliver/protobuf/rpcpb"
)

type stubRPC struct{}
var _ rpcpb.SliverRPCClient = (*stubRPC)(nil)

func (s *stubRPC) GetVersion(ctx context.Context, in *commonpb.Empty, opts ...grpc.CallOption) (*clientpb.Version, error) {
	return nil, nil
}

func (s *stubRPC) ClientLog(ctx context.Context, opts ...grpc.CallOption) (grpc.ClientStreamingClient[clientpb.ClientLogData, commonpb.Empty], error) {
	return nil, nil
}

func (s *stubRPC) GetOperators(ctx context.Context, in *commonpb.Empty, opts ...grpc.CallOption) (*clientpb.Operators, error) {
	return nil, nil
}

func (s *stubRPC) Kill(ctx context.Context, in *sliverpb.KillReq, opts ...grpc.CallOption) (*commonpb.Empty, error) {
	return nil, nil
}

func (s *stubRPC) Reconfigure(ctx context.Context, in *sliverpb.ReconfigureReq, opts ...grpc.CallOption) (*sliverpb.Reconfigure, error) {
	return nil, nil
}

func (s *stubRPC) Rename(ctx context.Context, in *clientpb.RenameReq, opts ...grpc.CallOption) (*commonpb.Empty, error) {
	return nil, nil
}

func (s *stubRPC) GetSessions(ctx context.Context, in *commonpb.Empty, opts ...grpc.CallOption) (*clientpb.Sessions, error) {
	return &clientpb.Sessions{}, nil
}

func (s *stubRPC) MonitorStart(ctx context.Context, in *commonpb.Empty, opts ...grpc.CallOption) (*commonpb.Response, error) {
	return nil, nil
}

func (s *stubRPC) MonitorStop(ctx context.Context, in *commonpb.Empty, opts ...grpc.CallOption) (*commonpb.Empty, error) {
	return nil, nil
}

func (s *stubRPC) MonitorListConfig(ctx context.Context, in *commonpb.Empty, opts ...grpc.CallOption) (*clientpb.MonitoringProviders, error) {
	return nil, nil
}

func (s *stubRPC) MonitorAddConfig(ctx context.Context, in *clientpb.MonitoringProvider, opts ...grpc.CallOption) (*commonpb.Response, error) {
	return nil, nil
}

func (s *stubRPC) MonitorDelConfig(ctx context.Context, in *clientpb.MonitoringProvider, opts ...grpc.CallOption) (*commonpb.Response, error) {
	return nil, nil
}

func (s *stubRPC) StartMTLSListener(ctx context.Context, in *clientpb.MTLSListenerReq, opts ...grpc.CallOption) (*clientpb.ListenerJob, error) {
	return nil, nil
}

func (s *stubRPC) StartWGListener(ctx context.Context, in *clientpb.WGListenerReq, opts ...grpc.CallOption) (*clientpb.ListenerJob, error) {
	return nil, nil
}

func (s *stubRPC) StartDNSListener(ctx context.Context, in *clientpb.DNSListenerReq, opts ...grpc.CallOption) (*clientpb.ListenerJob, error) {
	return nil, nil
}

func (s *stubRPC) StartHTTPSListener(ctx context.Context, in *clientpb.HTTPListenerReq, opts ...grpc.CallOption) (*clientpb.ListenerJob, error) {
	return nil, nil
}

func (s *stubRPC) StartHTTPListener(ctx context.Context, in *clientpb.HTTPListenerReq, opts ...grpc.CallOption) (*clientpb.ListenerJob, error) {
	return &clientpb.ListenerJob{}, nil
}

func (s *stubRPC) GetBeacons(ctx context.Context, in *commonpb.Empty, opts ...grpc.CallOption) (*clientpb.Beacons, error) {
	return nil, nil
}

func (s *stubRPC) GetBeacon(ctx context.Context, in *clientpb.Beacon, opts ...grpc.CallOption) (*clientpb.Beacon, error) {
	return nil, nil
}

func (s *stubRPC) RmBeacon(ctx context.Context, in *clientpb.Beacon, opts ...grpc.CallOption) (*commonpb.Empty, error) {
	return nil, nil
}

func (s *stubRPC) GetBeaconTasks(ctx context.Context, in *clientpb.Beacon, opts ...grpc.CallOption) (*clientpb.BeaconTasks, error) {
	return nil, nil
}

func (s *stubRPC) GetBeaconTaskContent(ctx context.Context, in *clientpb.BeaconTask, opts ...grpc.CallOption) (*clientpb.BeaconTask, error) {
	return nil, nil
}

func (s *stubRPC) CancelBeaconTask(ctx context.Context, in *clientpb.BeaconTask, opts ...grpc.CallOption) (*clientpb.BeaconTask, error) {
	return nil, nil
}

func (s *stubRPC) UpdateBeaconIntegrityInformation(ctx context.Context, in *clientpb.BeaconIntegrity, opts ...grpc.CallOption) (*commonpb.Empty, error) {
	return nil, nil
}

func (s *stubRPC) GetJobs(ctx context.Context, in *commonpb.Empty, opts ...grpc.CallOption) (*clientpb.Jobs, error) {
	return &clientpb.Jobs{}, nil
}

func (s *stubRPC) KillJob(ctx context.Context, in *clientpb.KillJobReq, opts ...grpc.CallOption) (*clientpb.KillJob, error) {
	return nil, nil
}

func (s *stubRPC) RestartJobs(ctx context.Context, in *clientpb.RestartJobReq, opts ...grpc.CallOption) (*commonpb.Empty, error) {
	return nil, nil
}

func (s *stubRPC) StartTCPStagerListener(ctx context.Context, in *clientpb.StagerListenerReq, opts ...grpc.CallOption) (*clientpb.StagerListener, error) {
	return nil, nil
}

func (s *stubRPC) LootAdd(ctx context.Context, in *clientpb.Loot, opts ...grpc.CallOption) (*clientpb.Loot, error) {
	return nil, nil
}

func (s *stubRPC) LootRm(ctx context.Context, in *clientpb.Loot, opts ...grpc.CallOption) (*commonpb.Empty, error) {
	return nil, nil
}

func (s *stubRPC) LootUpdate(ctx context.Context, in *clientpb.Loot, opts ...grpc.CallOption) (*clientpb.Loot, error) {
	return nil, nil
}

func (s *stubRPC) LootContent(ctx context.Context, in *clientpb.Loot, opts ...grpc.CallOption) (*clientpb.Loot, error) {
	return nil, nil
}

func (s *stubRPC) LootAll(ctx context.Context, in *commonpb.Empty, opts ...grpc.CallOption) (*clientpb.AllLoot, error) {
	return nil, nil
}

func (s *stubRPC) Creds(ctx context.Context, in *commonpb.Empty, opts ...grpc.CallOption) (*clientpb.Credentials, error) {
	return nil, nil
}

func (s *stubRPC) CredsAdd(ctx context.Context, in *clientpb.Credentials, opts ...grpc.CallOption) (*commonpb.Empty, error) {
	return nil, nil
}

func (s *stubRPC) CredsRm(ctx context.Context, in *clientpb.Credentials, opts ...grpc.CallOption) (*commonpb.Empty, error) {
	return nil, nil
}

func (s *stubRPC) CredsUpdate(ctx context.Context, in *clientpb.Credentials, opts ...grpc.CallOption) (*commonpb.Empty, error) {
	return nil, nil
}

func (s *stubRPC) GetCredByID(ctx context.Context, in *clientpb.Credential, opts ...grpc.CallOption) (*clientpb.Credential, error) {
	return nil, nil
}

func (s *stubRPC) GetCredsByHashType(ctx context.Context, in *clientpb.Credential, opts ...grpc.CallOption) (*clientpb.Credentials, error) {
	return nil, nil
}

func (s *stubRPC) GetPlaintextCredsByHashType(ctx context.Context, in *clientpb.Credential, opts ...grpc.CallOption) (*clientpb.Credentials, error) {
	return nil, nil
}

func (s *stubRPC) CredsSniffHashType(ctx context.Context, in *clientpb.Credential, opts ...grpc.CallOption) (*clientpb.Credential, error) {
	return nil, nil
}

func (s *stubRPC) Hosts(ctx context.Context, in *commonpb.Empty, opts ...grpc.CallOption) (*clientpb.AllHosts, error) {
	return nil, nil
}

func (s *stubRPC) Host(ctx context.Context, in *clientpb.Host, opts ...grpc.CallOption) (*clientpb.Host, error) {
	return nil, nil
}

func (s *stubRPC) HostRm(ctx context.Context, in *clientpb.Host, opts ...grpc.CallOption) (*commonpb.Empty, error) {
	return nil, nil
}

func (s *stubRPC) HostIOCRm(ctx context.Context, in *clientpb.IOC, opts ...grpc.CallOption) (*commonpb.Empty, error) {
	return nil, nil
}

func (s *stubRPC) Generate(ctx context.Context, in *clientpb.GenerateReq, opts ...grpc.CallOption) (*clientpb.Generate, error) {
	return nil, nil
}

func (s *stubRPC) GenerateSpoofMetadata(ctx context.Context, in *clientpb.GenerateSpoofMetadataReq, opts ...grpc.CallOption) (*commonpb.Empty, error) {
	return nil, nil
}

func (s *stubRPC) GenerateExternal(ctx context.Context, in *clientpb.ExternalGenerateReq, opts ...grpc.CallOption) (*clientpb.ExternalImplantConfig, error) {
	return nil, nil
}

func (s *stubRPC) GenerateExternalSaveBuild(ctx context.Context, in *clientpb.ExternalImplantBinary, opts ...grpc.CallOption) (*commonpb.Empty, error) {
	return nil, nil
}

func (s *stubRPC) GenerateExternalGetBuildConfig(ctx context.Context, in *clientpb.ImplantBuild, opts ...grpc.CallOption) (*clientpb.ExternalImplantConfig, error) {
	return nil, nil
}

func (s *stubRPC) GenerateStage(ctx context.Context, in *clientpb.GenerateStageReq, opts ...grpc.CallOption) (*clientpb.Generate, error) {
	return nil, nil
}

func (s *stubRPC) StageImplantBuild(ctx context.Context, in *clientpb.ImplantStageReq, opts ...grpc.CallOption) (*commonpb.Empty, error) {
	return nil, nil
}

func (s *stubRPC) GetHTTPC2Profiles(ctx context.Context, in *commonpb.Empty, opts ...grpc.CallOption) (*clientpb.HTTPC2Configs, error) {
	return nil, nil
}

func (s *stubRPC) GetHTTPC2ProfileByName(ctx context.Context, in *clientpb.C2ProfileReq, opts ...grpc.CallOption) (*clientpb.HTTPC2Config, error) {
	return nil, nil
}

func (s *stubRPC) SaveHTTPC2Profile(ctx context.Context, in *clientpb.HTTPC2ConfigReq, opts ...grpc.CallOption) (*commonpb.Empty, error) {
	return nil, nil
}

func (s *stubRPC) BuilderRegister(ctx context.Context, in *clientpb.Builder, opts ...grpc.CallOption) (grpc.ServerStreamingClient[clientpb.Event], error) {
	return nil, nil
}

func (s *stubRPC) BuilderTrigger(ctx context.Context, in *clientpb.Event, opts ...grpc.CallOption) (*commonpb.Empty, error) {
	return nil, nil
}

func (s *stubRPC) Builders(ctx context.Context, in *commonpb.Empty, opts ...grpc.CallOption) (*clientpb.Builders, error) {
	return nil, nil
}

func (s *stubRPC) GetCertificateInfo(ctx context.Context, in *clientpb.CertificatesReq, opts ...grpc.CallOption) (*clientpb.CertificateInfo, error) {
	return nil, nil
}

func (s *stubRPC) GetCertificateAuthorityInfo(ctx context.Context, in *commonpb.Empty, opts ...grpc.CallOption) (*clientpb.CertificateAuthorityInfo, error) {
	return nil, nil
}

func (s *stubRPC) Crack(ctx context.Context, in *clientpb.CrackCommand, opts ...grpc.CallOption) (*clientpb.CrackResponse, error) {
	return nil, nil
}

func (s *stubRPC) CrackstationRegister(ctx context.Context, in *clientpb.Crackstation, opts ...grpc.CallOption) (grpc.ServerStreamingClient[clientpb.Event], error) {
	return nil, nil
}

func (s *stubRPC) CrackstationTrigger(ctx context.Context, in *clientpb.Event, opts ...grpc.CallOption) (*commonpb.Empty, error) {
	return nil, nil
}

func (s *stubRPC) CrackstationBenchmark(ctx context.Context, in *clientpb.CrackBenchmark, opts ...grpc.CallOption) (*commonpb.Empty, error) {
	return nil, nil
}

func (s *stubRPC) Crackstations(ctx context.Context, in *commonpb.Empty, opts ...grpc.CallOption) (*clientpb.Crackstations, error) {
	return nil, nil
}

func (s *stubRPC) CrackTaskByID(ctx context.Context, in *clientpb.CrackTask, opts ...grpc.CallOption) (*clientpb.CrackTask, error) {
	return nil, nil
}

func (s *stubRPC) CrackTaskUpdate(ctx context.Context, in *clientpb.CrackTask, opts ...grpc.CallOption) (*commonpb.Empty, error) {
	return nil, nil
}

func (s *stubRPC) CrackFilesList(ctx context.Context, in *clientpb.CrackFile, opts ...grpc.CallOption) (*clientpb.CrackFiles, error) {
	return nil, nil
}

func (s *stubRPC) CrackFileCreate(ctx context.Context, in *clientpb.CrackFile, opts ...grpc.CallOption) (*clientpb.CrackFile, error) {
	return nil, nil
}

func (s *stubRPC) CrackFileChunkUpload(ctx context.Context, in *clientpb.CrackFileChunk, opts ...grpc.CallOption) (*commonpb.Empty, error) {
	return nil, nil
}

func (s *stubRPC) CrackFileChunkDownload(ctx context.Context, in *clientpb.CrackFileChunk, opts ...grpc.CallOption) (*clientpb.CrackFileChunk, error) {
	return nil, nil
}

func (s *stubRPC) CrackFileComplete(ctx context.Context, in *clientpb.CrackFile, opts ...grpc.CallOption) (*commonpb.Empty, error) {
	return nil, nil
}

func (s *stubRPC) CrackFileDelete(ctx context.Context, in *clientpb.CrackFile, opts ...grpc.CallOption) (*commonpb.Empty, error) {
	return nil, nil
}

func (s *stubRPC) Regenerate(ctx context.Context, in *clientpb.RegenerateReq, opts ...grpc.CallOption) (*clientpb.Generate, error) {
	return nil, nil
}

func (s *stubRPC) ImplantBuilds(ctx context.Context, in *commonpb.Empty, opts ...grpc.CallOption) (*clientpb.ImplantBuilds, error) {
	return &clientpb.ImplantBuilds{}, nil
}

func (s *stubRPC) DeleteImplantBuild(ctx context.Context, in *clientpb.DeleteReq, opts ...grpc.CallOption) (*commonpb.Empty, error) {
	return nil, nil
}

func (s *stubRPC) Canaries(ctx context.Context, in *commonpb.Empty, opts ...grpc.CallOption) (*clientpb.Canaries, error) {
	return nil, nil
}

func (s *stubRPC) GenerateWGClientConfig(ctx context.Context, in *commonpb.Empty, opts ...grpc.CallOption) (*clientpb.WGClientConfig, error) {
	return nil, nil
}

func (s *stubRPC) GenerateUniqueIP(ctx context.Context, in *commonpb.Empty, opts ...grpc.CallOption) (*clientpb.UniqueWGIP, error) {
	return nil, nil
}

func (s *stubRPC) ImplantProfiles(ctx context.Context, in *commonpb.Empty, opts ...grpc.CallOption) (*clientpb.ImplantProfiles, error) {
	return nil, nil
}

func (s *stubRPC) DeleteImplantProfile(ctx context.Context, in *clientpb.DeleteReq, opts ...grpc.CallOption) (*commonpb.Empty, error) {
	return nil, nil
}

func (s *stubRPC) SaveImplantProfile(ctx context.Context, in *clientpb.ImplantProfile, opts ...grpc.CallOption) (*clientpb.ImplantProfile, error) {
	return nil, nil
}

func (s *stubRPC) ShellcodeRDI(ctx context.Context, in *clientpb.ShellcodeRDIReq, opts ...grpc.CallOption) (*clientpb.ShellcodeRDI, error) {
	return nil, nil
}

func (s *stubRPC) GetCompiler(ctx context.Context, in *commonpb.Empty, opts ...grpc.CallOption) (*clientpb.Compiler, error) {
	return nil, nil
}

func (s *stubRPC) ShellcodeEncoder(ctx context.Context, in *clientpb.ShellcodeEncodeReq, opts ...grpc.CallOption) (*clientpb.ShellcodeEncode, error) {
	return nil, nil
}

func (s *stubRPC) ShellcodeEncoderMap(ctx context.Context, in *commonpb.Empty, opts ...grpc.CallOption) (*clientpb.ShellcodeEncoderMap, error) {
	return nil, nil
}

func (s *stubRPC) TrafficEncoderMap(ctx context.Context, in *commonpb.Empty, opts ...grpc.CallOption) (*clientpb.TrafficEncoderMap, error) {
	return nil, nil
}

func (s *stubRPC) TrafficEncoderAdd(ctx context.Context, in *clientpb.TrafficEncoder, opts ...grpc.CallOption) (*clientpb.TrafficEncoderTests, error) {
	return nil, nil
}

func (s *stubRPC) TrafficEncoderRm(ctx context.Context, in *clientpb.TrafficEncoder, opts ...grpc.CallOption) (*commonpb.Empty, error) {
	return nil, nil
}

func (s *stubRPC) Websites(ctx context.Context, in *commonpb.Empty, opts ...grpc.CallOption) (*clientpb.Websites, error) {
	return nil, nil
}

func (s *stubRPC) Website(ctx context.Context, in *clientpb.Website, opts ...grpc.CallOption) (*clientpb.Website, error) {
	return nil, nil
}

func (s *stubRPC) WebsiteRemove(ctx context.Context, in *clientpb.Website, opts ...grpc.CallOption) (*commonpb.Empty, error) {
	return nil, nil
}

func (s *stubRPC) WebsiteAddContent(ctx context.Context, in *clientpb.WebsiteAddContent, opts ...grpc.CallOption) (*clientpb.Website, error) {
	return nil, nil
}

func (s *stubRPC) WebsiteUpdateContent(ctx context.Context, in *clientpb.WebsiteAddContent, opts ...grpc.CallOption) (*clientpb.Website, error) {
	return nil, nil
}

func (s *stubRPC) WebsiteRemoveContent(ctx context.Context, in *clientpb.WebsiteRemoveContent, opts ...grpc.CallOption) (*clientpb.Website, error) {
	return nil, nil
}

func (s *stubRPC) Ping(ctx context.Context, in *sliverpb.Ping, opts ...grpc.CallOption) (*sliverpb.Ping, error) {
	return nil, nil
}

func (s *stubRPC) Ps(ctx context.Context, in *sliverpb.PsReq, opts ...grpc.CallOption) (*sliverpb.Ps, error) {
	return nil, nil
}

func (s *stubRPC) Terminate(ctx context.Context, in *sliverpb.TerminateReq, opts ...grpc.CallOption) (*sliverpb.Terminate, error) {
	return nil, nil
}

func (s *stubRPC) Ifconfig(ctx context.Context, in *sliverpb.IfconfigReq, opts ...grpc.CallOption) (*sliverpb.Ifconfig, error) {
	return nil, nil
}

func (s *stubRPC) Netstat(ctx context.Context, in *sliverpb.NetstatReq, opts ...grpc.CallOption) (*sliverpb.Netstat, error) {
	return nil, nil
}

func (s *stubRPC) Ls(ctx context.Context, in *sliverpb.LsReq, opts ...grpc.CallOption) (*sliverpb.Ls, error) {
	return nil, nil
}

func (s *stubRPC) Cd(ctx context.Context, in *sliverpb.CdReq, opts ...grpc.CallOption) (*sliverpb.Pwd, error) {
	return nil, nil
}

func (s *stubRPC) Pwd(ctx context.Context, in *sliverpb.PwdReq, opts ...grpc.CallOption) (*sliverpb.Pwd, error) {
	return nil, nil
}

func (s *stubRPC) Mv(ctx context.Context, in *sliverpb.MvReq, opts ...grpc.CallOption) (*sliverpb.Mv, error) {
	return nil, nil
}

func (s *stubRPC) Cp(ctx context.Context, in *sliverpb.CpReq, opts ...grpc.CallOption) (*sliverpb.Cp, error) {
	return nil, nil
}

func (s *stubRPC) Rm(ctx context.Context, in *sliverpb.RmReq, opts ...grpc.CallOption) (*sliverpb.Rm, error) {
	return nil, nil
}

func (s *stubRPC) Mkdir(ctx context.Context, in *sliverpb.MkdirReq, opts ...grpc.CallOption) (*sliverpb.Mkdir, error) {
	return nil, nil
}

func (s *stubRPC) Download(ctx context.Context, in *sliverpb.DownloadReq, opts ...grpc.CallOption) (*sliverpb.Download, error) {
	return nil, nil
}

func (s *stubRPC) Upload(ctx context.Context, in *sliverpb.UploadReq, opts ...grpc.CallOption) (*sliverpb.Upload, error) {
	return nil, nil
}

func (s *stubRPC) Grep(ctx context.Context, in *sliverpb.GrepReq, opts ...grpc.CallOption) (*sliverpb.Grep, error) {
	return nil, nil
}

func (s *stubRPC) Chmod(ctx context.Context, in *sliverpb.ChmodReq, opts ...grpc.CallOption) (*sliverpb.Chmod, error) {
	return nil, nil
}

func (s *stubRPC) Chown(ctx context.Context, in *sliverpb.ChownReq, opts ...grpc.CallOption) (*sliverpb.Chown, error) {
	return nil, nil
}

func (s *stubRPC) Chtimes(ctx context.Context, in *sliverpb.ChtimesReq, opts ...grpc.CallOption) (*sliverpb.Chtimes, error) {
	return nil, nil
}

func (s *stubRPC) MemfilesList(ctx context.Context, in *sliverpb.MemfilesListReq, opts ...grpc.CallOption) (*sliverpb.Ls, error) {
	return nil, nil
}

func (s *stubRPC) MemfilesAdd(ctx context.Context, in *sliverpb.MemfilesAddReq, opts ...grpc.CallOption) (*sliverpb.MemfilesAdd, error) {
	return nil, nil
}

func (s *stubRPC) MemfilesRm(ctx context.Context, in *sliverpb.MemfilesRmReq, opts ...grpc.CallOption) (*sliverpb.MemfilesRm, error) {
	return nil, nil
}

func (s *stubRPC) Mount(ctx context.Context, in *sliverpb.MountReq, opts ...grpc.CallOption) (*sliverpb.Mount, error) {
	return nil, nil
}

func (s *stubRPC) ProcessDump(ctx context.Context, in *sliverpb.ProcessDumpReq, opts ...grpc.CallOption) (*sliverpb.ProcessDump, error) {
	return nil, nil
}

func (s *stubRPC) RunAs(ctx context.Context, in *sliverpb.RunAsReq, opts ...grpc.CallOption) (*sliverpb.RunAs, error) {
	return nil, nil
}

func (s *stubRPC) Impersonate(ctx context.Context, in *sliverpb.ImpersonateReq, opts ...grpc.CallOption) (*sliverpb.Impersonate, error) {
	return nil, nil
}

func (s *stubRPC) RevToSelf(ctx context.Context, in *sliverpb.RevToSelfReq, opts ...grpc.CallOption) (*sliverpb.RevToSelf, error) {
	return nil, nil
}

func (s *stubRPC) GetSystem(ctx context.Context, in *clientpb.GetSystemReq, opts ...grpc.CallOption) (*sliverpb.GetSystem, error) {
	return nil, nil
}

func (s *stubRPC) Task(ctx context.Context, in *sliverpb.TaskReq, opts ...grpc.CallOption) (*sliverpb.Task, error) {
	return nil, nil
}

func (s *stubRPC) Msf(ctx context.Context, in *clientpb.MSFReq, opts ...grpc.CallOption) (*sliverpb.Task, error) {
	return nil, nil
}

func (s *stubRPC) MsfRemote(ctx context.Context, in *clientpb.MSFRemoteReq, opts ...grpc.CallOption) (*sliverpb.Task, error) {
	return nil, nil
}

func (s *stubRPC) ExecuteAssembly(ctx context.Context, in *sliverpb.ExecuteAssemblyReq, opts ...grpc.CallOption) (*sliverpb.ExecuteAssembly, error) {
	return nil, nil
}

func (s *stubRPC) Migrate(ctx context.Context, in *clientpb.MigrateReq, opts ...grpc.CallOption) (*sliverpb.Migrate, error) {
	return nil, nil
}

func (s *stubRPC) Execute(ctx context.Context, in *sliverpb.ExecuteReq, opts ...grpc.CallOption) (*sliverpb.Execute, error) {
	return nil, nil
}

func (s *stubRPC) ExecuteWindows(ctx context.Context, in *sliverpb.ExecuteWindowsReq, opts ...grpc.CallOption) (*sliverpb.Execute, error) {
	return nil, nil
}

func (s *stubRPC) ExecuteChildren(ctx context.Context, in *sliverpb.ExecuteChildrenReq, opts ...grpc.CallOption) (*sliverpb.ExecuteChildren, error) {
	return nil, nil
}

func (s *stubRPC) Sideload(ctx context.Context, in *sliverpb.SideloadReq, opts ...grpc.CallOption) (*sliverpb.Sideload, error) {
	return nil, nil
}

func (s *stubRPC) SpawnDll(ctx context.Context, in *sliverpb.InvokeSpawnDllReq, opts ...grpc.CallOption) (*sliverpb.SpawnDll, error) {
	return nil, nil
}

func (s *stubRPC) Screenshot(ctx context.Context, in *sliverpb.ScreenshotReq, opts ...grpc.CallOption) (*sliverpb.Screenshot, error) {
	return nil, nil
}

func (s *stubRPC) CurrentTokenOwner(ctx context.Context, in *sliverpb.CurrentTokenOwnerReq, opts ...grpc.CallOption) (*sliverpb.CurrentTokenOwner, error) {
	return nil, nil
}

func (s *stubRPC) Services(ctx context.Context, in *sliverpb.ServicesReq, opts ...grpc.CallOption) (*sliverpb.Services, error) {
	return nil, nil
}

func (s *stubRPC) ServiceDetail(ctx context.Context, in *sliverpb.ServiceDetailReq, opts ...grpc.CallOption) (*sliverpb.ServiceDetail, error) {
	return nil, nil
}

func (s *stubRPC) StartServiceByName(ctx context.Context, in *sliverpb.StartServiceByNameReq, opts ...grpc.CallOption) (*sliverpb.ServiceInfo, error) {
	return nil, nil
}

func (s *stubRPC) PivotStartListener(ctx context.Context, in *sliverpb.PivotStartListenerReq, opts ...grpc.CallOption) (*sliverpb.PivotListener, error) {
	return nil, nil
}

func (s *stubRPC) PivotStopListener(ctx context.Context, in *sliverpb.PivotStopListenerReq, opts ...grpc.CallOption) (*commonpb.Empty, error) {
	return nil, nil
}

func (s *stubRPC) PivotSessionListeners(ctx context.Context, in *sliverpb.PivotListenersReq, opts ...grpc.CallOption) (*sliverpb.PivotListeners, error) {
	return nil, nil
}

func (s *stubRPC) PivotGraph(ctx context.Context, in *commonpb.Empty, opts ...grpc.CallOption) (*clientpb.PivotGraph, error) {
	return nil, nil
}

func (s *stubRPC) StartService(ctx context.Context, in *sliverpb.StartServiceReq, opts ...grpc.CallOption) (*sliverpb.ServiceInfo, error) {
	return nil, nil
}

func (s *stubRPC) StopService(ctx context.Context, in *sliverpb.StopServiceReq, opts ...grpc.CallOption) (*sliverpb.ServiceInfo, error) {
	return nil, nil
}

func (s *stubRPC) RemoveService(ctx context.Context, in *sliverpb.RemoveServiceReq, opts ...grpc.CallOption) (*sliverpb.ServiceInfo, error) {
	return nil, nil
}

func (s *stubRPC) MakeToken(ctx context.Context, in *sliverpb.MakeTokenReq, opts ...grpc.CallOption) (*sliverpb.MakeToken, error) {
	return nil, nil
}

func (s *stubRPC) GetEnv(ctx context.Context, in *sliverpb.EnvReq, opts ...grpc.CallOption) (*sliverpb.EnvInfo, error) {
	return nil, nil
}

func (s *stubRPC) SetEnv(ctx context.Context, in *sliverpb.SetEnvReq, opts ...grpc.CallOption) (*sliverpb.SetEnv, error) {
	return nil, nil
}

func (s *stubRPC) UnsetEnv(ctx context.Context, in *sliverpb.UnsetEnvReq, opts ...grpc.CallOption) (*sliverpb.UnsetEnv, error) {
	return nil, nil
}

func (s *stubRPC) Backdoor(ctx context.Context, in *clientpb.BackdoorReq, opts ...grpc.CallOption) (*clientpb.Backdoor, error) {
	return nil, nil
}

func (s *stubRPC) RegistryRead(ctx context.Context, in *sliverpb.RegistryReadReq, opts ...grpc.CallOption) (*sliverpb.RegistryRead, error) {
	return nil, nil
}

func (s *stubRPC) RegistryWrite(ctx context.Context, in *sliverpb.RegistryWriteReq, opts ...grpc.CallOption) (*sliverpb.RegistryWrite, error) {
	return nil, nil
}

func (s *stubRPC) RegistryCreateKey(ctx context.Context, in *sliverpb.RegistryCreateKeyReq, opts ...grpc.CallOption) (*sliverpb.RegistryCreateKey, error) {
	return nil, nil
}

func (s *stubRPC) RegistryDeleteKey(ctx context.Context, in *sliverpb.RegistryDeleteKeyReq, opts ...grpc.CallOption) (*sliverpb.RegistryDeleteKey, error) {
	return nil, nil
}

func (s *stubRPC) RegistryListSubKeys(ctx context.Context, in *sliverpb.RegistrySubKeyListReq, opts ...grpc.CallOption) (*sliverpb.RegistrySubKeyList, error) {
	return nil, nil
}

func (s *stubRPC) RegistryListValues(ctx context.Context, in *sliverpb.RegistryListValuesReq, opts ...grpc.CallOption) (*sliverpb.RegistryValuesList, error) {
	return nil, nil
}

func (s *stubRPC) RegistryReadHive(ctx context.Context, in *sliverpb.RegistryReadHiveReq, opts ...grpc.CallOption) (*sliverpb.RegistryReadHive, error) {
	return nil, nil
}

func (s *stubRPC) RunSSHCommand(ctx context.Context, in *sliverpb.SSHCommandReq, opts ...grpc.CallOption) (*sliverpb.SSHCommand, error) {
	return nil, nil
}

func (s *stubRPC) HijackDLL(ctx context.Context, in *clientpb.DllHijackReq, opts ...grpc.CallOption) (*clientpb.DllHijack, error) {
	return nil, nil
}

func (s *stubRPC) GetPrivs(ctx context.Context, in *sliverpb.GetPrivsReq, opts ...grpc.CallOption) (*sliverpb.GetPrivs, error) {
	return nil, nil
}

func (s *stubRPC) StartRportFwdListener(ctx context.Context, in *sliverpb.RportFwdStartListenerReq, opts ...grpc.CallOption) (*sliverpb.RportFwdListener, error) {
	return nil, nil
}

func (s *stubRPC) GetRportFwdListeners(ctx context.Context, in *sliverpb.RportFwdListenersReq, opts ...grpc.CallOption) (*sliverpb.RportFwdListeners, error) {
	return nil, nil
}

func (s *stubRPC) StopRportFwdListener(ctx context.Context, in *sliverpb.RportFwdStopListenerReq, opts ...grpc.CallOption) (*sliverpb.RportFwdListener, error) {
	return nil, nil
}

func (s *stubRPC) OpenSession(ctx context.Context, in *sliverpb.OpenSession, opts ...grpc.CallOption) (*sliverpb.OpenSession, error) {
	return nil, nil
}

func (s *stubRPC) CloseSession(ctx context.Context, in *sliverpb.CloseSession, opts ...grpc.CallOption) (*commonpb.Empty, error) {
	return nil, nil
}

func (s *stubRPC) RegisterExtension(ctx context.Context, in *sliverpb.RegisterExtensionReq, opts ...grpc.CallOption) (*sliverpb.RegisterExtension, error) {
	return nil, nil
}

func (s *stubRPC) CallExtension(ctx context.Context, in *sliverpb.CallExtensionReq, opts ...grpc.CallOption) (*sliverpb.CallExtension, error) {
	return nil, nil
}

func (s *stubRPC) ListExtensions(ctx context.Context, in *sliverpb.ListExtensionsReq, opts ...grpc.CallOption) (*sliverpb.ListExtensions, error) {
	return nil, nil
}

func (s *stubRPC) RegisterWasmExtension(ctx context.Context, in *sliverpb.RegisterWasmExtensionReq, opts ...grpc.CallOption) (*sliverpb.RegisterWasmExtension, error) {
	return nil, nil
}

func (s *stubRPC) ListWasmExtensions(ctx context.Context, in *sliverpb.ListWasmExtensionsReq, opts ...grpc.CallOption) (*sliverpb.ListWasmExtensions, error) {
	return nil, nil
}

func (s *stubRPC) ExecWasmExtension(ctx context.Context, in *sliverpb.ExecWasmExtensionReq, opts ...grpc.CallOption) (*sliverpb.ExecWasmExtension, error) {
	return nil, nil
}

func (s *stubRPC) WGStartPortForward(ctx context.Context, in *sliverpb.WGPortForwardStartReq, opts ...grpc.CallOption) (*sliverpb.WGPortForward, error) {
	return nil, nil
}

func (s *stubRPC) WGStopPortForward(ctx context.Context, in *sliverpb.WGPortForwardStopReq, opts ...grpc.CallOption) (*sliverpb.WGPortForward, error) {
	return nil, nil
}

func (s *stubRPC) WGStartSocks(ctx context.Context, in *sliverpb.WGSocksStartReq, opts ...grpc.CallOption) (*sliverpb.WGSocks, error) {
	return nil, nil
}

func (s *stubRPC) WGStopSocks(ctx context.Context, in *sliverpb.WGSocksStopReq, opts ...grpc.CallOption) (*sliverpb.WGSocks, error) {
	return nil, nil
}

func (s *stubRPC) WGListForwarders(ctx context.Context, in *sliverpb.WGTCPForwardersReq, opts ...grpc.CallOption) (*sliverpb.WGTCPForwarders, error) {
	return nil, nil
}

func (s *stubRPC) WGListSocksServers(ctx context.Context, in *sliverpb.WGSocksServersReq, opts ...grpc.CallOption) (*sliverpb.WGSocksServers, error) {
	return nil, nil
}

func (s *stubRPC) Shell(ctx context.Context, in *sliverpb.ShellReq, opts ...grpc.CallOption) (*sliverpb.Shell, error) {
	return nil, nil
}

func (s *stubRPC) ShellResize(ctx context.Context, in *sliverpb.ShellResizeReq, opts ...grpc.CallOption) (*commonpb.Empty, error) {
	return nil, nil
}

func (s *stubRPC) Portfwd(ctx context.Context, in *sliverpb.PortfwdReq, opts ...grpc.CallOption) (*sliverpb.Portfwd, error) {
	return nil, nil
}

func (s *stubRPC) CreateSocks(ctx context.Context, in *sliverpb.Socks, opts ...grpc.CallOption) (*sliverpb.Socks, error) {
	return nil, nil
}

func (s *stubRPC) CloseSocks(ctx context.Context, in *sliverpb.Socks, opts ...grpc.CallOption) (*commonpb.Empty, error) {
	return nil, nil
}

func (s *stubRPC) SocksProxy(ctx context.Context, opts ...grpc.CallOption) (grpc.BidiStreamingClient[sliverpb.SocksData, sliverpb.SocksData], error) {
	return nil, nil
}

func (s *stubRPC) CreateTunnel(ctx context.Context, in *sliverpb.Tunnel, opts ...grpc.CallOption) (*sliverpb.Tunnel, error) {
	return nil, nil
}

func (s *stubRPC) CloseTunnel(ctx context.Context, in *sliverpb.Tunnel, opts ...grpc.CallOption) (*commonpb.Empty, error) {
	return nil, nil
}

func (s *stubRPC) TunnelData(ctx context.Context, opts ...grpc.CallOption) (grpc.BidiStreamingClient[sliverpb.TunnelData, sliverpb.TunnelData], error) {
	return nil, nil
}

func (s *stubRPC) Events(ctx context.Context, in *commonpb.Empty, opts ...grpc.CallOption) (grpc.ServerStreamingClient[clientpb.Event], error) {
	return nil, nil
}

