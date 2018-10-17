package terraform

import (
	"fmt"
	"sort"

	"github.com/hashicorp/terraform/addrs"
	"github.com/hashicorp/terraform/states"
)

// StateFilter is responsible for filtering and searching a state.
//
// This is a separate struct from State rather than a method on State
// because StateFilter might create sidecar data structures to optimize
// filtering on the state.
//
// If you change the State, the filter created is invalid and either
// Reset should be called or a new one should be allocated. StateFilter
// will not watch State for changes and do this for you. If you filter after
// changing the State without calling Reset, the behavior is not defined.
type StateFilter struct {
	State *states.State
}

// Filter takes the addresses specified by fs and finds all the matches.
// The values of fs are resource addressing syntax that can be parsed by
// ParseResourceAddress.
func (f *StateFilter) Filter(fs ...string) ([]*StateFilterResult, error) {
	// Parse all the addresses
	as := make([]addrs.Targetable, len(fs))
	for i, v := range fs {
		if addr, diags := addrs.ParseModuleInstanceStr(v); !diags.HasErrors() {
			as[i] = addr
			continue
		}
		if addr, diags := addrs.ParseAbsResourceStr(v); !diags.HasErrors() {
			as[i] = addr
			continue
		}
		if addr, diags := addrs.ParseAbsResourceInstanceStr(v); !diags.HasErrors() {
			as[i] = addr
			continue
		}
		return nil, fmt.Errorf("Error parsing address '%s'", v)
	}

	// If we weren't given any filters, then we list all
	if len(fs) == 0 {
		as = append(as, addrs.RootModuleInstance)
	}

	// Filter each of the address. We keep track of this in a map to
	// strip duplicates.
	resultSet := make(map[string]*StateFilterResult)
	for _, addr := range as {
		for _, r := range f.filterSingle(addr) {
			resultSet[r.String()] = r
		}
	}

	// Make the result list
	results := make([]*StateFilterResult, 0, len(resultSet))
	for _, v := range resultSet {
		results = append(results, v)
	}

	// Sort them and return
	sort.Sort(StateFilterResultSlice(results))
	return results, nil
}

func (f *StateFilter) filterSingle(addr addrs.Targetable) []*StateFilterResult {
	// The slice to keep track of results
	var results []*StateFilterResult

	// Check if we received a module instance address that
	// should be used as module filter, and if not set the
	// filter to the root module instance.
	filter, ok := addr.(addrs.ModuleInstance)
	if !ok {
		filter = addrs.RootModuleInstance
	}

	// Go through modules first.
	modules := make([]*states.Module, 0, len(f.State.Modules))
	for _, m := range f.State.Modules {
		if filter.IsRoot() || filter.Equal(m.Addr) {
			modules = append(modules, m)

			// Only add the module to the results if we searched
			// for a specific non-root module and found a match.
			if !filter.IsRoot() && filter.Equal(m.Addr) {
				results = append(results, &StateFilterResult{
					Address: m.Addr.String(),
					Value:   m,
				})
			}
		}
	}

	// With the modules set, go through all the resources within
	// the modules to find relevant resources.
	for _, m := range modules {
		for _, r := range m.Resources {
			for key, is := range r.Instances {
				if f.relevant(addr, r.Addr.Absolute(m.Addr), key) {
					results = append(results, &StateFilterResult{
						Address: r.Addr.Absolute(m.Addr).Instance(key).String(),
						Value:   is,
					})
				}
			}
		}
	}

	return results
}

func (f *StateFilter) relevant(filter addrs.Targetable, addr addrs.AbsResource, key addrs.InstanceKey) bool {
	switch filter := filter.(type) {
	case addrs.AbsResource:
		if filter.Module != nil {
			return filter.Equal(addr)
		}
		return filter.Resource.Equal(addr.Resource)
	case addrs.AbsResourceInstance:
		if filter.Module != nil {
			return filter.Equal(addr.Instance(key))
		}
		return filter.Resource.Equal(addr.Resource.Instance(key))
	default:
		return true
	}
}

// StateFilterResult is a single result from a filter operation. Filter can
// match multiple things within a state (curently modules and resources).
type StateFilterResult struct {
	// Address is the address that can be used to reference this exact result.
	Address string

	// Value is the actual value. This must be type switched on. It can be
	// any either a `states.Module` or `states.ResourceInstance`.
	Value interface{}
}

func (r *StateFilterResult) String() string {
	return fmt.Sprintf("%T: %s", r.Value, r.Address)
}

func (r *StateFilterResult) sortedType() int {
	switch r.Value.(type) {
	case *states.Module:
		return 0
	case *states.ResourceInstance:
		return 1
	default:
		return 50
	}
}

// StateFilterResultSlice is a slice of results that implements
// sort.Interface. The sorting goal is what is most appealing to
// human output.
type StateFilterResultSlice []*StateFilterResult

func (s StateFilterResultSlice) Len() int      { return len(s) }
func (s StateFilterResultSlice) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s StateFilterResultSlice) Less(i, j int) bool {
	a, b := s[i], s[j]

	// If the addresses are different it is just lexographic sorting
	if a.Address != b.Address {
		return a.Address < b.Address
	}

	// Addresses are the same, which means it matters on the type
	return a.sortedType() < b.sortedType()
}
