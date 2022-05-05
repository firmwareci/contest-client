package env

const (
	EnvLogFile             = "FIRMWARECI_LOGFILE"
	DefaultLogFile         = "/logs/job.result"
	DefaultBinaryDir       = "/root/assets/testbin/"
	EnvBinUrl              = "FIRMWARECI_BINURL"
	EnvBinDir              = "FIRMWARECI_BINDIR"
	EnvSSHHostPublic       = "FIRMWARECI_HOSTPUB"
	EnvSSHPrivatePW        = "FIRMWARECI_SSHPRIVATEPW"
	EnvS3Access            = "FIRMWARECI_S3_ACCESS"
	EnvS3Secret            = "FIRMWARECI_S3_SECRET"
	EnvS3Region            = "FIRMWARECI_S3_REGION"
	FirmwareciSSHPublicKey = "/root/.ssh/firmwareci.pub"
)
