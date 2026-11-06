package tui

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/versaDeploy/internal/config"
	"github.com/user/versaDeploy/internal/deployer"
	"github.com/user/versaDeploy/internal/logger"
	"github.com/user/versaDeploy/internal/provision"
	versassh "github.com/user/versaDeploy/internal/ssh"
)

type msgServicesDetected struct {
	envName string
	report  *provision.Report
	err     error
	certExp time.Time // certificate served on domain:443 (zero when unchecked/failed)
	certErr error
}
type msgProvisionLine struct{ line string }
type msgProvisionDone struct{ err error }

// svcField is one editable install parameter.
type svcField struct {
	label   string
	get     func(provision.Vars) string
	set     func(*provision.Vars, string)
	secret  bool
	product string // php | node | python: Enter cycles the supported versions, e types one
}

// versionChoices are the options of a version field: supported versions newest first,
// plus the current value when it isn't among them (e.g. an older one on the server).
// Python starts with "" = the system python3.
func versionChoices(product, current string) []string {
	opts := provision.Versions(product)
	if product == "python" {
		opts = append([]string{""}, opts...)
	}
	if !slices.Contains(opts, current) {
		opts = append(opts, current)
	}
	return opts
}

var svcFields = []svcField{
	{label: "Domain", get: func(v provision.Vars) string { return v.Domain }, set: func(v *provision.Vars, s string) { v.Domain = s }},
	{label: "Aliases", get: func(v provision.Vars) string { return strings.Join(v.Aliases, " ") }, set: func(v *provision.Vars, s string) {
		v.Aliases = strings.FieldsFunc(s, func(r rune) bool { return r == ' ' || r == ',' })
	}},
	{label: "Admin email", get: func(v provision.Vars) string { return v.AdminEmail }, set: func(v *provision.Vars, s string) { v.AdminEmail = s }},
	{label: "PHP version", product: "php", get: func(v provision.Vars) string { return v.PHPVersion }, set: func(v *provision.Vars, s string) { v.PHPVersion = s }},
	{label: "Python version", product: "python", get: func(v provision.Vars) string { return v.PythonVersion }, set: func(v *provision.Vars, s string) { v.PythonVersion = s }},
	{label: "Doc root", get: func(v provision.Vars) string { return v.DocRoot }, set: func(v *provision.Vars, s string) { v.DocRoot = s }},
	{label: "Deploy user", get: func(v provision.Vars) string { return v.DeployUser }, set: func(v *provision.Vars, s string) { v.DeployUser = s }},
	{label: "DB name", get: func(v provision.Vars) string { return v.DBName }, set: func(v *provision.Vars, s string) { v.DBName = s }},
	{label: "DB user", get: func(v provision.Vars) string { return v.DBUser }, set: func(v *provision.Vars, s string) { v.DBUser = s }},
	{label: "DB password", secret: true, get: func(v provision.Vars) string { return v.DBPassword }, set: func(v *provision.Vars, s string) { v.DBPassword = s }},
	{label: "Proxy port", get: func(v provision.Vars) string {
		if v.ProxyPort == 0 {
			return ""
		}
		return strconv.Itoa(v.ProxyPort)
	}, set: func(v *provision.Vars, s string) { v.ProxyPort, _ = strconv.Atoi(s) }},
	{label: "App service", get: func(v provision.Vars) string { return v.AppService }, set: func(v *provision.Vars, s string) { v.AppService = s }},
	{label: "Backup keep", get: func(v provision.Vars) string {
		if v.BackupKeep == 0 {
			return ""
		}
		return strconv.Itoa(v.BackupKeep)
	}, set: func(v *provision.Vars, s string) { v.BackupKeep, _ = strconv.Atoi(s) }},
	{label: "Node major", product: "node", get: func(v provision.Vars) string { return v.NodeMajor }, set: func(v *provision.Vars, s string) { v.NodeMajor = s }},
}

type servicesModel struct {
	envName  string
	report   *provision.Report
	backends []provision.Backend
	vars     provision.Vars
	selected map[string]bool
	loaded   bool
	loading  bool
	certExp  time.Time
	certErr  error
	env      *config.Environment // active environment, set on detection
	project  string
	err      error
	status   string

	cursor  int // 0..len(Catalog)-1 services, then fields
	editing bool
	input   textinput.Model

	// confirm modal: what will run when the user presses Y
	confirmTitle string
	confirmLines []string
	pending      adminJob

	running bool
	done    bool
	runErr  error
	logBuf  *strings.Builder
	logCh   chan string
	errCh   chan error
	vp      viewport.Model
	w, h    int
}

func newServicesModel(w, h int) servicesModel {
	ti := textinput.New()
	ti.CharLimit = 255
	return servicesModel{selected: map[string]bool{}, input: ti, logBuf: &strings.Builder{}, w: w, h: h}
}

func (s *servicesModel) setSize(w, h int) {
	s.w, s.h = w, h
	s.vp = viewport.New(max(w-4, 10), max(h-10, 4))
	s.vp.SetContent(s.logBuf.String())
}

func (s servicesModel) rows() int { return len(provision.Catalog) + len(svcFields) }

// detectServicesCmd probes the server. Without a deploy connection (fresh server, deploy
// user not created yet) it uses the admin connection.
func detectServicesCmd(client *versassh.Client, env *config.Environment, envName, project string) tea.Cmd {
	params := provision.DetectParams{
		Project:    project,
		RemotePath: env.RemotePath,
		DeployUser: env.Provision.DeployUser,
	}
	if len(env.Services) > 0 {
		params.AppService = env.Services[0].Name
	}
	if params.DeployUser == "" {
		params.DeployUser = env.SSH.User
	}
	admin := env.ProvisionSSH()
	domain := env.Provision.Domain
	return func() tea.Msg {
		c := client
		if c == nil {
			log, _ := logger.NewLogger("", false, false)
			var err error
			if c, err = versassh.NewClient(&admin, log); err != nil {
				return msgServicesDetected{envName: envName, err: fmt.Errorf("no connection (deploy or admin): %w", err)}
			}
			defer c.Close()
		}
		r, err := provision.Detect(c, params)
		msg := msgServicesDetected{envName: envName, report: r, err: err}
		if domain != "" {
			msg.certExp, msg.certErr = provision.CertExpiry(domain)
		}
		return msg
	}
}

func (s *servicesModel) applyDetected(msg msgServicesDetected, env *config.Environment, project, repoPath string) {
	s.env, s.project = env, project
	s.loading = false
	s.loaded = true
	s.err = msg.err
	if msg.err != nil {
		return
	}
	first := s.report == nil || s.envName != msg.envName
	s.envName = msg.envName
	s.report = msg.report
	s.certExp, s.certErr = msg.certExp, msg.certErr
	if first {
		s.backends = provision.DetectBackends(env, repoPath)
		s.vars = provision.Defaults(env, project, s.backends, s.report)
		s.selected = map[string]bool{}
		for _, id := range provision.Preselect(s.backends, s.vars, env, s.report) {
			s.selected[id] = true
		}
	} else {
		// after an install keep the user's parameters; drop what is now present
		for id := range s.selected {
			if s.report.Get(id).Status != provision.Absent {
				delete(s.selected, id)
			}
		}
	}
	if pub, err := os.ReadFile(env.SSH.KeyPath + ".pub"); err == nil {
		s.vars.PubKey = string(pub)
	}
}

func (s *servicesModel) toggle() {
	if s.cursor >= len(provision.Catalog) {
		return
	}
	svc := provision.Catalog[s.cursor]
	if svc.ID == "app_service" {
		s.status = StyleHint.Render("Python/Go/Node apps run as deploy.yml `services`: each deploy installs and restarts them (w adds the entry)")
		return
	}
	if s.selected[svc.ID] {
		delete(s.selected, svc.ID)
		return
	}
	if err := provision.Conflicts(s.report, []string{svc.ID}, s.vars); err != nil {
		s.status = StyleError.Render("✕ " + err.Error())
		return
	}
	if svc.Group != "" {
		for _, o := range provision.Catalog {
			if o.Group == svc.Group {
				delete(s.selected, o.ID)
			}
		}
	}
	s.selected[svc.ID] = true
	switch svc.Group {
	case "web":
		s.vars.WebServer = svc.ID
	case "db":
		s.vars.Database = svc.ID
	}
}

func (s *servicesModel) selectedIDs() []string {
	var out []string
	for _, svc := range provision.Catalog {
		if s.selected[svc.ID] {
			out = append(out, svc.ID)
		}
	}
	return out
}

// cycleVersion moves a version field to its next option.
func (s *servicesModel) cycleVersion() bool {
	f := svcFields[s.cursor-len(provision.Catalog)]
	if f.product == "" {
		return false
	}
	opts := versionChoices(f.product, f.get(s.vars))
	i := slices.Index(opts, f.get(s.vars))
	f.set(&s.vars, opts[(i+1)%len(opts)])
	return true
}

func (s *servicesModel) startEdit() {
	f := svcFields[s.cursor-len(provision.Catalog)]
	s.editing = true
	s.input.SetValue(f.get(s.vars))
	s.input.EchoMode = textinput.EchoNormal
	if f.secret {
		s.input.EchoMode = textinput.EchoPassword
	}
	s.input.CursorEnd()
	s.input.Focus()
}

func (s *servicesModel) finishEdit(save bool) {
	if save {
		svcFields[s.cursor-len(provision.Catalog)].set(&s.vars, strings.TrimSpace(s.input.Value()))
	}
	s.editing = false
	s.input.Blur()
}

// askInstall prepares the confirm modal for the selection.
func (s *servicesModel) askInstall() {
	s.askSteps(s.selectedIDs(), "Install / configure on "+s.envName)
}

// askPrepare readies a server for deploying: the deploy user with this machine's key,
// its sudoers for services_reload and the project directory.
func (s *servicesModel) askPrepare() {
	s.askSteps([]string{"deploy_user", "sudoers", "dirs"}, "Prepare "+s.envName+" for deploy (user "+s.vars.DeployUser+")")
	if s.pending != nil {
		s.confirmLines = append(s.confirmLines, "", StyleHint.Render("  Then: Operations → Initial deploy → D"))
	}
}

// askResetPassword sets the DB user's password explicitly: by default an existing user
// keeps it, so other apps using that user don't lose access.
func (s *servicesModel) askResetPassword() {
	id := s.vars.Database
	if id != "postgres" && id != "mariadb" {
		s.status = StyleWarning.Render("Select PostgreSQL or MariaDB/MySQL first")
		return
	}
	v := s.vars
	v.ResetDBPassword = true
	saved := s.vars
	s.vars = v
	s.askSteps([]string{id}, "RESET password of DB user "+v.DBUser)
	s.vars = saved
	if s.pending != nil {
		s.confirmLines = append(s.confirmLines, "", StyleError.Render("  Every app using "+v.DBUser+" must get the new password (.env)"))
	}
}

func (s *servicesModel) askSteps(ids []string, title string) {
	s.pending = nil
	if !s.report.Apt {
		s.status = StyleError.Render("Install needs an apt-based server (Debian/Ubuntu)")
		return
	}
	if err := provision.Conflicts(s.report, ids, s.vars); err != nil {
		s.status = StyleError.Render("✕ " + err.Error())
		return
	}
	v := s.vars
	if slices.Contains(ids, "sudoers") && s.env != nil {
		// let deploys install and restart the app's services as the deploy user
		e := *s.env
		svcs, err := servicesForDeployUser(s.env, s.vars)
		if err != nil {
			s.status = StyleError.Render("✕ " + err.Error())
			return
		}
		e.Services = svcs
		if len(e.Services) > 0 {
			e.SSH.User = s.vars.DeployUser
			v.ServicesSudoers = deployer.SudoersScript(&e, s.project)
			inst, err := deployer.ServiceInstallScript(&e, s.project, s.envName)
			if err != nil {
				s.status = StyleError.Render("✕ " + err.Error())
				return
			}
			v.ServicesInstall = inst
		}
	}
	b, err := provision.Build(ids, v)
	if err != nil {
		s.status = StyleError.Render("✕ " + err.Error())
		return
	}
	s.pending = bundleJob(b)
	s.confirmTitle = title
	s.confirmLines = nil
	for _, id := range ids {
		svc, _ := provision.Lookup(id)
		s.confirmLines = append(s.confirmLines, "• "+svc.Label)
	}
	if r := s.report; r.ProjectSite != "" && (slices.Contains(ids, "nginx") || slices.Contains(ids, "apache")) {
		s.confirmLines = append(s.confirmLines, "", StyleWarning.Render("  This app is already served by "+r.ProjectSite+": the install stops if it"),
			StyleWarning.Render("  serves the same domain (it is never overwritten)"))
	}
	if slices.Contains(ids, "ufw") {
		s.confirmLines = append(s.confirmLines, "", StyleWarning.Render(fmt.Sprintf("  UFW: allows SSH %d, 80 and 443 (an active firewall keeps its policy)", s.vars.SSHPort)))
	}
}

// servicesForDeployUser returns the services the sudoers step installs for the deploy
// user. The rules go to that user, so services that ran as the old ssh.user run as
// the deploy user, and none may run as root: the deploy user owns the wrapper root
// would execute.
func servicesForDeployUser(env *config.Environment, v provision.Vars) ([]config.ServiceConfig, error) {
	svcs := slices.Clone(provision.ProposedServices(env, v))
	for i := range svcs {
		if u := svcs[i].User; u == "" || u == env.SSH.User {
			svcs[i].User = v.DeployUser
		}
		if svcs[i].User == "root" && v.DeployUser != "root" {
			return nil, fmt.Errorf("service %s runs as root: set its user to %s", svcs[i].Name, v.DeployUser)
		}
	}
	return svcs, nil
}

// askUpgrade prepares the confirm modal for upgrades: one service, security only, or
// just refreshing the apt index.
func (s *servicesModel) askUpgrade(kind string) {
	if !s.report.Apt {
		s.status = StyleError.Render("Upgrades need an apt-based server (Debian/Ubuntu)")
		return
	}
	var ups []provision.Upgrade
	var units []string
	switch kind {
	case "service":
		if s.cursor >= len(provision.Catalog) {
			return
		}
		svc := provision.Catalog[s.cursor]
		ups = s.report.UpgradesFor(svc.ID)
		units = svc.UnitNames(s.report.PHPVersion)
		s.confirmTitle = "Upgrade " + svc.Label
	case "security":
		ups = s.report.SecurityUpgrades()
		for _, svc := range provision.Catalog {
			if len(s.report.UpgradesFor(svc.ID)) > 0 && s.report.Get(svc.ID).Status == provision.Active {
				units = append(units, svc.UnitNames(s.report.PHPVersion)...)
			}
		}
		s.confirmTitle = "Apply security updates"
	case "refresh":
		s.pending = bundleJob(provision.UpgradeBundle(nil, nil, true))
		s.confirmTitle = "Refresh package index (apt-get update)"
		s.confirmLines = nil
		return
	}
	if len(ups) == 0 {
		s.status = StyleMuted.Render("No pending upgrades")
		return
	}
	s.confirmLines = nil
	pkgs := make([]string, 0, len(ups))
	for _, u := range ups {
		pkgs = append(pkgs, u.Pkg)
		s.confirmLines = append(s.confirmLines, fmt.Sprintf("• %-28s %s → %s", u.Pkg, u.From, u.To))
	}
	s.pending = bundleJob(provision.UpgradeBundle(pkgs, units, false))
}

func (s *servicesModel) startRun() {
	s.running, s.done, s.runErr = true, false, nil
	s.logBuf = &strings.Builder{}
	s.logCh = make(chan string, 256)
	s.errCh = make(chan error, 1)
	s.vp = viewport.New(max(s.w-4, 10), max(s.h-10, 4))
}

func (s *servicesModel) appendLog(chunk string) {
	s.logBuf.WriteString(strings.ReplaceAll(chunk, "\r", ""))
	s.vp.SetContent(s.logBuf.String())
	s.vp.GotoBottom()
}

// adminJob runs on the admin connection, writing its output to w.
type adminJob func(c *versassh.Client, user string, w io.Writer) error

func bundleJob(b provision.Bundle) adminJob {
	return func(c *versassh.Client, user string, w io.Writer) error { return provision.Run(c, user, b, w) }
}

func execJob(cmd string) adminJob {
	return func(c *versassh.Client, user string, w io.Writer) error { return provision.Exec(c, user, cmd, w) }
}

// runAdminCmd opens the admin connection and runs job, streaming its output.
func runAdminCmd(sshCfg config.SSHConfig, job adminJob, ch chan string, errCh chan error) tea.Cmd {
	return func() tea.Msg {
		go func() {
			w := &logCapture{ch: ch}
			log := logger.NewTUILogger(w, true, false)
			client, err := versassh.NewClient(&sshCfg, log)
			if err == nil {
				ch <- fmt.Sprintf("[INFO]  Connected as %s@%s\n", sshCfg.User, sshCfg.Host)
				err = job(client, sshCfg.User, w)
				client.Close()
			}
			if err != nil {
				ch <- fmt.Sprintf("\n[ERROR] %v\n", err)
			}
			errCh <- err
			close(ch)
		}()
		return waitProvision(ch, errCh)()
	}
}

func waitProvision(ch <-chan string, errCh <-chan error) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-ch
		if !ok {
			return msgProvisionDone{err: <-errCh}
		}
		return msgProvisionLine{line: line}
	}
}

// missingReload returns the suggested services_reload commands not yet in env.
func (s servicesModel) missingReload(env *config.Environment) []string {
	var out []string
	for _, c := range provision.SuggestedReload(s.vars) {
		if !slices.Contains(env.ServicesReload, c) {
			out = append(out, c)
		}
	}
	return out
}

// ── rendering ────────────────────────────────────────────────────────────────

func (s servicesModel) view(width, height int, env *config.Environment) string {
	if env.Local {
		return StyleMuted.Render("\n  Services needs a remote environment (this one is local: true)")
	}
	if !s.loaded {
		return StyleMuted.Render("\n  Detecting services…")
	}
	if s.err != nil {
		return StyleError.Render("\n  Error: "+errDisplay(s.err)) + "\n\n" + StyleMuted.Render("  F5 retry   c reconnect   (set provision.ssh in deploy.yml for a fresh server)")
	}
	sep := StyleMuted.Render(strings.Repeat("─", max(width-4, 4)))
	if s.running || s.done {
		state := StyleWarning.Render("\U000F0765 running…")
		foot := StyleMuted.Render("  ↑↓ scroll   (running…)")
		if s.done {
			state = StyleSuccess.Render("\U000F012C done")
			if s.runErr != nil {
				state = StyleError.Render("\U000F0159 failed")
			}
			foot = StyleMuted.Render("  Esc close log   ↑↓ scroll")
		}
		return strings.Join([]string{"", StyleTitle.Render("  Services") + "  " + state, "", sep, s.vp.View(), sep, foot}, "\n")
	}
	if s.confirmTitle != "" {
		rows := []string{"", StyleWarning.Render("  ⚠  " + s.confirmTitle), "", sep, ""}
		for _, l := range s.confirmLines {
			rows = append(rows, "  "+l)
		}
		user := env.ProvisionSSH().User
		rows = append(rows, "", sep, "",
			StyleHint.Render("  Runs as "+user+"@"+env.SSH.Host+" (provision.ssh)"),
			"", fmt.Sprintf("  %s Proceed   %s Cancel", StyleCmd.Render("[Y]"), StyleCmd.Render("[N]")))
		return strings.Join(rows, "\n")
	}

	r := s.report
	head := []string{"", StyleTitle.Render("  Services") + "  " + StyleMuted.Render(osLabel(r))}
	head = append(head, s.alerts()...)
	if len(s.backends) > 0 {
		var parts []string
		for _, b := range s.backends {
			parts = append(parts, strings.TrimSpace(strings.ToUpper(b.Kind)+" "+b.Detail)+" ("+b.Source+")")
		}
		head = append(head, StyleHint.Render("  Backend: "+strings.Join(parts, " · ")))
	} else {
		head = append(head, StyleMuted.Render("  Backend: not detected"))
	}
	if r.Get("deploy_user").Status == provision.Absent && s.vars.DeployUser != "" {
		head = append(head, StyleWarning.Render("  ⚠ Deploy user "+s.vars.DeployUser+" does not exist yet — P prepares the server for deploy (user + your key + sudoers + dir)"))
	}
	if l := s.sslLine(env); l != "" {
		head = append(head, l)
	}
	head = append(head, "")

	var body []string
	cursorLine := 0
	for i, svc := range provision.Catalog {
		if i == s.cursor {
			cursorLine = len(body)
		}
		body = append(body, s.serviceRow(i, svc))
	}
	body = append(body, "", StyleSection.Render("  Parameters"))
	for i, f := range svcFields {
		idx := len(provision.Catalog) + i
		val := f.get(s.vars)
		if f.secret && val != "" {
			val = strings.Repeat("•", 8)
		}
		if f.product != "" {
			val = renderChoices(versionChoices(f.product, f.get(s.vars)), f.get(s.vars))
		} else if val == "" {
			val = StyleMuted.Render("—")
		}
		if s.editing && idx == s.cursor {
			val = s.input.View()
		}
		line := fmt.Sprintf("    %-14s %s", f.label+":", val)
		if idx == s.cursor {
			cursorLine = len(body)
			if !s.editing {
				line = StyleSelected.Render(fmt.Sprintf("  > %-14s", f.label+":")) + " " + val
			}
		}
		body = append(body, line)
	}
	if len(r.Sites) > 0 {
		body = append(body, "", StyleSection.Render(fmt.Sprintf("  Sites on this server (%d)", len(r.Sites))))
		for _, site := range r.Sites {
			mark := "  "
			if strings.TrimSuffix(site.File, ".conf") == s.vars.Project || site.File == r.ProjectSite {
				mark = StyleSuccess.Render("▸ ")
			}
			body = append(body, fmt.Sprintf("  %s%-28s %s", mark, truncate(site.Names, 28), StyleMuted.Render(site.Root)))
		}
	}
	if miss := s.missingReload(env); len(miss) > 0 {
		body = append(body, "", StyleWarning.Render("  services_reload is missing: "+strings.Join(miss, ", ")+"   [a] add"))
	}

	foot := []string{}
	if s.status != "" {
		foot = append(foot, "  "+s.status)
	}
	foot = append(foot, sep, StyleMuted.Render("  Space select  ↵ edit  I install  P prepare deploy  U upgrade  S security  ctrl+r apt  L logs  B backup  X reset DB pass  w save  a reload"))

	// window the body around the cursor so long lists fit
	avail := height - len(head) - len(foot)
	if avail < 5 {
		avail = 5
	}
	start := 0
	if len(body) > avail {
		start = min(max(cursorLine-avail/2, 0), len(body)-avail)
		body = body[start : start+avail]
	}
	return strings.Join(append(append(head, body...), foot...), "\n")
}

func (s servicesModel) serviceRow(i int, svc provision.Service) string {
	st := s.report.Get(svc.ID)
	check := StyleMuted.Render("[ ]")
	if s.selected[svc.ID] {
		check = StyleSuccess.Render("[\U000F012C]")
	}
	var dot, label string
	switch st.Status {
	case provision.Active:
		dot, label = StyleSuccess.Render("●"), StyleSuccess.Render("active   ")
	case provision.Installed:
		if len(svc.Units) > 0 && svc.ID != "ufw" {
			dot, label = StyleWarning.Render("●"), StyleWarning.Render("stopped  ")
		} else {
			dot, label = StyleSuccess.Render("●"), StyleSuccess.Render("installed")
		}
	default:
		dot, label = StyleMuted.Render("○"), StyleMuted.Render("absent   ")
	}
	extra := StyleMuted.Render(truncate(st.Version, 22))
	if svc.ID == "backup" && s.report.BackupLast != "" {
		extra = StyleMuted.Render("last: " + s.report.BackupLast)
	}
	if ups := s.report.UpgradesFor(svc.ID); len(ups) > 0 && st.Status != provision.Absent {
		sec := false
		for _, u := range ups {
			sec = sec || u.Security
		}
		up := fmt.Sprintf("↑ %d update(s) → %s", len(ups), truncate(ups[0].To, 20))
		if sec {
			extra += "  " + StyleError.Render(up+" security")
		} else {
			extra += "  " + StyleWarning.Render(up)
		}
	}
	name := fmt.Sprintf("%-34s", svc.Label)
	if i == s.cursor {
		return fmt.Sprintf("  %s %s ", check, dot) + StyleSelected.Render(name) + " " + label + "  " + extra
	}
	return fmt.Sprintf("  %s %s %s %s  %s", check, dot, name, label, extra)
}

// alerts returns the security banner lines (also shown on the Dashboard).
func (s servicesModel) alerts() []string {
	if s.report == nil {
		return nil
	}
	return securityAlerts(s.report)
}

func securityAlerts(r *provision.Report) []string {
	if r == nil || !r.Apt {
		return nil
	}
	var parts []string
	if n := len(r.SecurityUpgrades()); n > 0 {
		parts = append(parts, fmt.Sprintf("%d security update(s) pending", n))
	}
	if r.RebootRequired {
		parts = append(parts, "reboot required")
	}
	if r.Unattended == "inactive" {
		parts = append(parts, "unattended-upgrades off")
	}
	var out []string
	if len(parts) > 0 {
		out = append(out, StyleError.Render("  ⚠ "+strings.Join(parts, " · ")))
	}
	if r.AptAge > 7*24*3600 {
		out = append(out, StyleWarning.Render(fmt.Sprintf("  ⚠ package index is %d days old — ctrl+r to refresh before trusting update info", r.AptAge/86400)))
	}
	return out
}

func osLabel(r *provision.Report) string {
	l := r.OSID
	if r.Apt {
		l += " · apt"
	} else {
		l += " · detection only (no apt)"
	}
	return strings.TrimPrefix(l, " · ")
}

func truncate(s string, n int) string {
	if len([]rune(s)) <= n {
		return s
	}
	return string([]rune(s)[:n-1]) + "…"
}

// ── app wiring ───────────────────────────────────────────────────────────────

func (m appModel) servicesDetectCmds() []tea.Cmd {
	env := m.activeEnvCfg()
	if env == nil || env.Local {
		return nil
	}
	fetch := func() tea.Msg { provision.FetchVersions(); return nil } // once per process
	return []tea.Cmd{detectServicesCmd(m.activeClient(), env, m.activeEnvName(), m.cfg.Project), fetch}
}

func (m *appModel) handleServicesKey(msg tea.KeyMsg) []tea.Cmd {
	s := &m.services
	env := m.activeEnvCfg()
	if env == nil || s.report == nil {
		return nil
	}
	k := msg.String()

	if s.editing {
		switch k {
		case "enter":
			s.finishEdit(true)
		case "esc":
			s.finishEdit(false)
		default:
			var cmd tea.Cmd
			s.input, cmd = s.input.Update(msg)
			return []tea.Cmd{cmd}
		}
		return nil
	}

	if s.confirmTitle != "" {
		switch k {
		case "y", "Y":
			b := s.pending
			s.confirmTitle, s.pending = "", nil
			s.startRun()
			return []tea.Cmd{runAdminCmd(env.ProvisionSSH(), b, s.logCh, s.errCh)}
		case "n", "N", "esc":
			s.confirmTitle, s.pending = "", nil
		}
		return nil
	}

	if s.running || s.done {
		switch k {
		case "up":
			s.vp.LineUp(3)
		case "down":
			s.vp.LineDown(3)
		case "pgup":
			s.vp.ViewUp()
		case "pgdown":
			s.vp.ViewDown()
		case "esc":
			if s.done {
				s.done = false
			}
		}
		return nil
	}

	s.status = ""
	switch k {
	case "up":
		if s.cursor > 0 {
			s.cursor--
		}
	case "down":
		if s.cursor < s.rows()-1 {
			s.cursor++
		}
	case " ":
		s.toggle()
	case "enter":
		if s.cursor >= len(provision.Catalog) {
			if !s.cycleVersion() {
				s.startEdit()
			}
		} else {
			s.toggle()
		}
	case "I", "i":
		s.askInstall()
	case "e":
		if s.cursor >= len(provision.Catalog) {
			s.startEdit()
		}
	case "P":
		s.askPrepare()
	case "X":
		s.askResetPassword()
	case "U":
		s.askUpgrade("service")
	case "S":
		s.askUpgrade("security")
	case "ctrl+r":
		s.askUpgrade("refresh")
	case "L":
		if s.cursor < len(provision.Catalog) {
			svc := provision.Catalog[s.cursor]
			s.startRun()
			s.appendLog("Logs: " + svc.Label + "\n")
			return []tea.Cmd{runAdminCmd(env.ProvisionSSH(), execJob(provision.LogsCommand(svc.ID, s.vars, s.report.PHPVersion)), s.logCh, s.errCh)}
		}
	case "B":
		if s.report.Get("backup").Status == provision.Absent {
			s.status = StyleWarning.Render("Install 'DB backups' first (select it and press I)")
			return nil
		}
		s.startRun()
		return []tea.Cmd{runAdminCmd(env.ProvisionSSH(), execJob(provision.BackupNowCommand(m.cfg.Project)), s.logCh, s.errCh)}
	case "w":
		p := s.vars.Apply(env.Provision, s.selectedIDs(), s.backends)
		if err := config.SaveProvision(m.config.configPath, m.activeEnvName(), p); err != nil {
			s.status = StyleError.Render("✕ " + err.Error())
			return nil
		}
		msg := "✓ provision saved to " + m.config.configPath
		if len(env.Services) == 0 {
			if svcs := provision.ProposedServices(env, s.vars); len(svcs) > 0 {
				if err := config.SetServices(m.config.configPath, m.activeEnvName(), svcs); err != nil {
					s.status = StyleError.Render("✕ " + err.Error())
					return nil
				}
				msg += " (+ services: " + svcs[0].Name + ")"
			}
		}
		m.reloadConfig()
		s.status = StyleSuccess.Render(msg)
	case "a":
		miss := s.missingReload(env)
		if len(miss) == 0 {
			s.status = StyleMuted.Render("services_reload already complete")
			return nil
		}
		if err := config.AddServicesReload(m.config.configPath, m.activeEnvName(), miss); err != nil {
			s.status = StyleError.Render("✕ " + err.Error())
			return nil
		}
		m.reloadConfig()
		s.status = StyleSuccess.Render("✓ services_reload updated: " + strings.Join(miss, ", "))
	}
	return nil
}

// reloadConfig re-reads the config file after the Services view edited it.
func (m *appModel) reloadConfig() {
	if cfg, err := config.Load(m.config.configPath); err == nil {
		m.cfg = cfg
		m.config.loaded = false
	} else {
		m.statusMsg = "Reload failed: " + errDisplay(err)
	}
}

// sslLine describes the certificate the domain serves (checked from this machine).
func (s servicesModel) sslLine(env *config.Environment) string {
	if env.Provision.Domain == "" {
		return ""
	}
	if s.certErr != nil {
		if env.Provision.SSL {
			return StyleError.Render("  ⚠ SSL " + env.Provision.Domain + ": " + truncate(s.certErr.Error(), 80))
		}
		return StyleMuted.Render("  SSL: not served on " + env.Provision.Domain + ":443")
	}
	if s.certExp.IsZero() {
		return ""
	}
	days := int(time.Until(s.certExp).Hours() / 24)
	l := fmt.Sprintf("  SSL %s: valid until %s (%d days)", env.Provision.Domain, s.certExp.Format("2006-01-02"), days)
	if days < 14 {
		return StyleError.Render("  ⚠" + l[1:] + " — check certbot.timer")
	}
	return StyleSuccess.Render(l)
}

// renderChoices shows every option with the current one highlighted: [8.4].
func renderChoices(opts []string, current string) string {
	parts := make([]string, len(opts))
	for i, o := range opts {
		label := o
		if label == "" {
			label = "system"
		}
		if o == current {
			parts[i] = StyleSuccess.Render("[" + label + "]")
		} else {
			parts[i] = StyleMuted.Render(label)
		}
	}
	return strings.Join(parts, " ") + StyleMuted.Render("  (↵ next · e type)")
}
