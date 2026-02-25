package contract

type SSHRunner interface {
	Run(cmd string) (stdout, stderr string, err error)
	RunWithStdin(stdin string, cmd string) (stdout, stderr string, err error)
}

type SSHClient interface {
	SSHRunner
	MkdirAllSudoWithPerm(path, perm string) error
	UploadFileSudo(localPath, remotePath string) error
	UploadFileSudoWithPerm(localPath, remotePath, perm string) error
	Close() error
}
