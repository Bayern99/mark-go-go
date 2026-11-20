package feynman

type ResearchFocusEvent struct {
	Root string `json:"root"`
}

type CLIKeyValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type CLISection struct {
	Title   string        `json:"title"`
	Entries []CLIKeyValue `json:"entries"`
}

type CLICommandResult struct {
	Command   string `json:"command"`
	Parsed    bool   `json:"parsed"`
	RawOutput string `json:"rawOutput"`
	ExitCode  int    `json:"exitCode"`
	Error     string `json:"error,omitempty"`
	Summary   string `json:"summary,omitempty"`
}

type CLIStatusResult struct {
	Command       string       `json:"command"`
	Parsed        bool         `json:"parsed"`
	RawOutput     string       `json:"rawOutput"`
	ExitCode      int          `json:"exitCode"`
	Error         string       `json:"error,omitempty"`
	Sections      []CLISection `json:"sections"`
	Model         string       `json:"model,omitempty"`
	ModelValid    string       `json:"modelValid,omitempty"`
	AlphaStatus   string       `json:"alphaStatus,omitempty"`
	WebAccess     string       `json:"webAccess,omitempty"`
	ServiceTier   string       `json:"serviceTier,omitempty"`
	PreviewStatus string       `json:"previewStatus,omitempty"`
	SessionsDir   string       `json:"sessionsDir,omitempty"`
	PiRuntime     string       `json:"piRuntime,omitempty"`
}

type CLIDoctorResult struct {
	Command   string        `json:"command"`
	Parsed    bool          `json:"parsed"`
	RawOutput string        `json:"rawOutput"`
	ExitCode  int           `json:"exitCode"`
	Error     string        `json:"error,omitempty"`
	Entries   []CLIKeyValue `json:"entries"`
	Issues    []string      `json:"issues"`
	Healthy   bool          `json:"healthy"`
}

type CLIModelEntry struct {
	ID          string `json:"id"`
	Provider    string `json:"provider"`
	Name        string `json:"name"`
	Current     bool   `json:"current"`
	Recommended bool   `json:"recommended"`
}

type CLIModelListResult struct {
	Command          string          `json:"command"`
	Parsed           bool            `json:"parsed"`
	RawOutput        string          `json:"rawOutput"`
	ExitCode         int             `json:"exitCode"`
	Error            string          `json:"error,omitempty"`
	Models           []CLIModelEntry `json:"models"`
	CurrentModel     string          `json:"currentModel,omitempty"`
	RecommendedModel string          `json:"recommendedModel,omitempty"`
	Tier             string          `json:"tier,omitempty"`
}

type CLISearchStatusResult struct {
	Command        string        `json:"command"`
	Parsed         bool          `json:"parsed"`
	RawOutput      string        `json:"rawOutput"`
	ExitCode       int           `json:"exitCode"`
	Error          string        `json:"error,omitempty"`
	ManagedBy      string        `json:"managedBy,omitempty"`
	SearchRoute    string        `json:"searchRoute,omitempty"`
	RequestRoute   string        `json:"requestRoute,omitempty"`
	SearchWorkflow string        `json:"searchWorkflow,omitempty"`
	SavedAPIKeys   string        `json:"savedApiKeys,omitempty"`
	ConfigPath     string        `json:"configPath,omitempty"`
	Configured     bool          `json:"configured"`
	Entries        []CLIKeyValue `json:"entries"`
}

type CLIMCPServer struct {
	Name      string   `json:"name"`
	Scope     string   `json:"scope,omitempty"`
	Transport string   `json:"transport,omitempty"`
	URL       string   `json:"url,omitempty"`
	Command   string   `json:"command,omitempty"`
	Exposure  string   `json:"exposure,omitempty"`
	Status    string   `json:"status,omitempty"`
	Error     string   `json:"error,omitempty"`
	Tools     []string `json:"tools,omitempty"`
}

type CLIMCPListResult struct {
	Command   string         `json:"command"`
	Parsed    bool           `json:"parsed"`
	RawOutput string         `json:"rawOutput"`
	ExitCode  int            `json:"exitCode"`
	Error     string         `json:"error,omitempty"`
	Servers   []CLIMCPServer `json:"servers"`
	Errors    []string       `json:"errors"`
}

type CLIMCPAddInput struct {
	Name              string   `json:"name"`
	Transport         string   `json:"transport"`
	URL               string   `json:"url,omitempty"`
	Command           string   `json:"command,omitempty"`
	Args              []string `json:"args,omitempty"`
	Headers           []string `json:"headers,omitempty"`
	Env               []string `json:"env,omitempty"`
	Cwd               string   `json:"cwd,omitempty"`
	BearerTokenEnvVar string   `json:"bearerTokenEnvVar,omitempty"`
	Exposure          string   `json:"exposure,omitempty"`
	Description       string   `json:"description,omitempty"`
	Local             bool     `json:"local,omitempty"`
}

type CLIPackageEntry struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Kind        string `json:"kind"`
	Installed   bool   `json:"installed"`
}

type CLIPackageListResult struct {
	Command   string            `json:"command"`
	Parsed    bool              `json:"parsed"`
	RawOutput string            `json:"rawOutput"`
	ExitCode  int               `json:"exitCode"`
	Error     string            `json:"error,omitempty"`
	Packages  []CLIPackageEntry `json:"packages"`
}

type CLIAlphaStatusResult struct {
	Command    string `json:"command"`
	Parsed     bool   `json:"parsed"`
	RawOutput  string `json:"rawOutput"`
	ExitCode   int    `json:"exitCode"`
	Error      string `json:"error,omitempty"`
	LoggedIn   bool   `json:"loggedIn"`
	Account    string `json:"account,omitempty"`
	StatusText string `json:"statusText,omitempty"`
}

type CLIAlphaPaper struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Authors   string `json:"authors,omitempty"`
	Published string `json:"published,omitempty"`
	URL       string `json:"url,omitempty"`
	Summary   string `json:"summary,omitempty"`
	Snippet   string `json:"snippet,omitempty"`
}

type CLIAlphaSearchResult struct {
	Command   string                `json:"command"`
	Parsed    bool                  `json:"parsed"`
	RawOutput string                `json:"rawOutput"`
	ExitCode  int                   `json:"exitCode"`
	Error     string                `json:"error,omitempty"`
	Query     string                `json:"query"`
	Mode      string                `json:"mode"`
	Papers    []CLIAlphaPaper       `json:"papers"`
	Groups    []CLIAlphaSearchGroup `json:"groups,omitempty"`
}

type CLIAlphaSearchGroup struct {
	Mode   string          `json:"mode"`
	Papers []CLIAlphaPaper `json:"papers"`
}

type CLIAlphaPaperResult struct {
	Command    string `json:"command"`
	Parsed     bool   `json:"parsed"`
	RawOutput  string `json:"rawOutput"`
	ExitCode   int    `json:"exitCode"`
	Error      string `json:"error,omitempty"`
	PaperID    string `json:"paperId"`
	Title      string `json:"title,omitempty"`
	Content    string `json:"content,omitempty"`
	Annotation string `json:"annotation,omitempty"`
	FullText   bool   `json:"fullText"`
}

type CLIAlphaAnnotation struct {
	PaperID   string `json:"paperId"`
	Note      string `json:"note"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

type CLIAlphaAnnotateResult struct {
	Command     string               `json:"command"`
	Parsed      bool                 `json:"parsed"`
	RawOutput   string               `json:"rawOutput"`
	ExitCode    int                  `json:"exitCode"`
	Error       string               `json:"error,omitempty"`
	PaperID     string               `json:"paperId,omitempty"`
	Note        string               `json:"note,omitempty"`
	Annotations []CLIAlphaAnnotation `json:"annotations"`
}

type CLITerminalLaunchResult struct {
	Wizard   string `json:"wizard"`
	Command  string `json:"command"`
	Launched bool   `json:"launched"`
}
