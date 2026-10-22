/*
	the protocol definiton of the job API
	the package jobapi is the channel between steps and the runner: HTTP+JSON over
	a unix socket that only exists while the job runs

		GET		/env			list entries (secret values are not included)
		GET		/env/{name}		one entry, secret resolved
		PUT		/env/{name}		set a literal	body: {"value": "..."}
		DELETE	/env/{name}		unset
		POST	/load			load a config file	body: {"path", "profile", "export"}

		debug it without the client:
			curl --unix-socket "QAHYA_JOB_API" http://qahya/env
*/
package jobapi

// env vars the runner sets for every step
const (
	SocketEnv = "QAHYA_JOB_API" // where the client finds the socket
	StepEnv = "QAHYA_STEP" // the step name, sent back so the store records who set what
)

const stepHeader = "Qahya-Step"

type Entry struct {
	Name   	string 		`json:"name"`
	Value  	string 		`json:"value,omitempty"`
	Secret 	bool   		`json:"secret,omitempty"`
	Ref    	string 		`json:"ref,omitempty"` // secret reference, e.g. local:db/prod#password
	Source 	string 		`json:"source,omitempty"`
}

type setRequest struct {
	Value string `json:"value"`
}

type loadRequest struct {
	Path    string `json:"path"` // must be absolute: the runner's working dir is not the step's
	Profile string `json:"profile,omitempty"`
	Export  bool   `json:"export,omitempty"` // include resolved values so the client can print export lines
}

type loadResponse struct {
	Entries []Entry `json:"entries"`
}

type errorResponse struct {
	Error string `json:"error"`
}