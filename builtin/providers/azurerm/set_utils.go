package azurerm

import (
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform/helper/hashcode"
	"github.com/hashicorp/terraform/helper/schema"
)

// setElementField is a collection of parameters relating to the
// field of a set's element, both in Terraform and its json rendition.
type setElementField struct {
	// schemaName and jsonName are the two strings which identify the field's
	// name within Terraform's schema, and the json name, respectively:
	schemaName, jsonName string

	// typ is the schema.Type representing the type of the field:
	typ schema.ValueType

	// isSubResource is a flag which indicates whether or not the field
	// is of a subresource type. Note that this is only applicable for
	// fields of type string or list of strings:
	isSubResource bool
}

// extractSet is a dirty hack which takes an existing *schema.Set based off
// of which it creates a new *schema.Set with respect to a provided list of
// JSON-serializable items and a list of field specifications.
// The process is plain and simply: marshall and then unmarshall v into a list,
// for each element of said list, look for the fields in {fields.jsonName} and
// put them in the respective {fields.schemaName} of a new set element to be
// found in the returned *schema.Set.
func extractSet(s *schema.Set, v interface{}, fields []setElementField) *schema.Set {
	set := schema.NewSet(s.F, nil)

	// first off; simply return an empty set right off the bat:
	if v == nil {
		return set
	}

	jso, err := json.Marshal(v)
	if err != nil {
		// if the provided value is a cyclic data structure or unmarshallable
		// for whatever reason; it is a fatal error on the part of the caller:
		panic(err)
	}

	// unmarshal the json into a []interface{}:
	var vals []interface{}

	// NOTE: if the JSON package can't unmarshal something it just marshalled
	// then all faith in humanity must be immediately abandoned:
	_ = json.Unmarshal(jso, &vals)

	// now; iterate through all of the unmarshalled elements and add them to said set:
	m := map[string]interface{}{}
	for i := range vals {
		elem := vals[i].(map[string]interface{})

		// NOTE: the "name" field is treated differently:
		if name, ok := elem["name"]; ok {
			m["name"] = name.(string)
		}

		// NOTE: absolutely all configurable things in the ARM API have a
		// specific "properties" field within them, if not, it is a fatal
		// error on the part of the caller:
		elem = elem["properties"].(map[string]interface{})

		// iterate through all the expected fields:
		for _, field := range fields {
			var ok bool
			var val interface{}

			// if field is not present, continue to the next one:
			if val, ok = elem[field.jsonName]; !ok {
				continue
			}

			// switch depending on the type:
			switch field.typ {
			case schema.TypeString:
				if field.isSubResource {
					val = val.(map[string]interface{})["id"].(string)
				} else {
					val = val.(string)
				}
			case schema.TypeInt:
				// NOTE: all json numbers are unmarshalled into float64,
				// so we must cast it to an int here:
				val = int(val.(float64))
			case schema.TypeBool:
				val = val.(bool)
			case schema.TypeList:
				// NOTE: can only be a list of strings or a list of references:
				if field.isSubResource {
					vs := []string{}
					for _, s := range val.([]interface{}) {
						vs = append(vs, s.(map[string]interface{})["id"].(string))
					}
					val = vs
				} else {
					val = val.([]string)
				}
			default:
				panic(fmt.Sprintf("bad type: %v", field.typ))
			}

			// finally; add the element to the map:
			m[field.schemaName] = val
		}

		// add the element to the set and reset the helper map:
		set.Add(m)
		m = map[string]interface{}{}
	}

	return set
}

// makeHashFunction is a helper function which; given a slice of field names
// and a list of the names
// returns a schema.SchemaSetFunc
func makeHashFunction(simpleFields []string, listFields []string) schema.SchemaSetFunc {
	return func(v interface{}) int {
		m := v.(map[string]interface{})
		s := ""

		// first; fetch the simple fields:
		for _, field := range simpleFields {
			if val, ok := m[field]; ok {
				// NOTE: some fields may hold integers or booleans:
				s = s + fmt.Sprintf("%v", val)
			}
		}

		// then; fetch the list fields:
		for _, field := range listFields {
			if val, ok := m[field]; ok {
				for _, item := range val.([]interface{}) {
					s = s + fmt.Sprintf("%v", item)
				}
			}
		}

		return hashcode.String(s)
	}
}

// readSet is a helper function which; given a *schema.Set and a slice with
// the name of the list fields which are treated specially and returns
// a slice of maps for each element of the set.
func readSet(v interface{}, listFieldNames []string) []map[string]interface{} {
	set := v.(*schema.Set)

	// check if set is empty:
	if set.Len() == 0 {
		return nil
	}

	// iterate through the elements, translate it into a map and add them all:
	res := []map[string]interface{}{}
	for _, elem := range set.List() {
		mp := map[string]interface{}{}

		// iterate through each element:
		for key, val := range elem.(map[string]interface{}) {
			// check if it is a list field:
			if in(key, listFieldNames) {
				list := []string{}

				for _, v := range val.([]interface{}) {
					list = append(list, v.(string))
				}

				mp[key] = list
			} else {
				// else, it's a simple field (either string or int),
				// so we just add it as-is:
				mp[key] = val
			}
		}
	}

	return res
}

// in is a helper function which determines whether or not a given string
// is present in the given slice of strings.
func in(str string, strs []string) bool {
	if strs == nil || len(strs) == 0 {
		return false
	}

	for _, s := range strs {
		if str == s {
			return true
		}
	}

	return false
}
