package provision

import "github.com/user/versaDeploy/internal/config"

// Apply merges v and the selection into p (the environment's provision block), keeping
// fields the Services view doesn't edit (ssh). db_password is left as is.
func (v Vars) Apply(p config.ProvisionConfig, selected []string, backends []Backend) config.ProvisionConfig {
	has := func(id string) bool {
		for _, s := range selected {
			if s == id {
				return true
			}
		}
		return false
	}
	p.Domain, p.WebServer, p.DocRoot, p.Database = v.Domain, v.WebServer, v.DocRoot, v.Database
	p.DBName, p.DBUser, p.AdminEmail, p.DeployUser = v.DBName, v.DBUser, v.AdminEmail, v.DeployUser
	p.ProxyPort, p.NodeMajor = v.ProxyPort, v.NodeMajor
	if v.Backend == "php" || v.ProxyPort == 0 {
		p.PHPVersion = v.PHPVersion
	}
	p.Redis = (p.Redis || has("redis")) && !has("dragonfly")
	p.Dragonfly = (p.Dragonfly || has("dragonfly")) && !has("redis")
	p.FrankenPHP = (p.FrankenPHP || has("frankenphp")) && !has("php")
	p.SSL = p.SSL || has("ssl")
	p.UFW = p.UFW || has("ufw")
	p.Fail2ban = p.Fail2ban || has("fail2ban")
	p.Backup = p.Backup || has("backup")
	p.Aliases, p.BackupKeep, p.PythonVersion = v.Aliases, v.BackupKeep, v.PythonVersion
	p.Backends = nil
	for _, b := range backends {
		p.Backends = append(p.Backends, b.Kind)
	}
	if p.DocRoot == DefaultDocRoot {
		p.DocRoot = "" // default
	}
	return p
}
