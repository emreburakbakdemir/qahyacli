// package provider fetches secret values from wherever they live. only
// a file provider exists for now; vault, 1password or an exec provier
// would be more implementations of the same interface
package provider

// provider looks up one secret. for the reference "local:db/prod#password"
// path is ["db", "prod"] and field is "password"
type Provider interface {
	Lookup(path []string, field string) (string, error)
}