package node

import (
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/tailscale/hujson"
)

// Config is the unified node configuration file (AST-backed for comment preservation)
type Config struct {
	ast *hujson.Value
}

// Helper: getObjectField returns a field from an object by name
func getObjectField(v *hujson.Value, name string) *hujson.Value {
	if v == nil || v.Value == nil {
		return &hujson.Value{}
	}
	obj, ok := v.Value.(*hujson.Object)
	if !ok {
		return &hujson.Value{}
	}
	for _, m := range obj.Members {
		if lit, ok := m.Name.Value.(hujson.Literal); ok && lit.String() == name {
			return &m.Value
		}
	}
	return &hujson.Value{}
}

// Helper: setStringField sets a string field on an object
func setStringField(v *hujson.Value, name string, value string) {
	if v == nil {
		return
	}
	obj, ok := v.Value.(*hujson.Object)
	if !ok {
		return
	}

	// Look for existing member
	for i, m := range obj.Members {
		if lit, ok := m.Name.Value.(hujson.Literal); ok && lit.String() == name {
			if value == "" {
				// Remove the field
				obj.Members = append(obj.Members[:i], obj.Members[i+1:]...)
			} else {
				// Update existing member
				m.Value.Value = hujson.String(value)
				obj.Members[i] = m
			}
			return
		}
	}

	// Add new member
	obj.Members = append(obj.Members, hujson.ObjectMember{
		Name:  hujson.Value{Value: hujson.String(name)},
		Value: hujson.Value{Value: hujson.String(value)},
	})
}

// Helper: fieldToString safely converts a field value to string
func fieldToString(v *hujson.Value) string {
	if v == nil || v.Value == nil {
		return ""
	}
	if lit, ok := v.Value.(hujson.Literal); ok && len(lit) > 0 {
		return lit.String()
	}
	return ""
}

// Helper: fieldToInt64 safely converts a field value to int64
func fieldToInt64(v *hujson.Value) int64 {
	if v == nil || v.Value == nil {
		return 0
	}
	if lit, ok := v.Value.(hujson.Literal); ok && len(lit) > 0 {
		return lit.Int()
	}
	return 0
}

// RelayAddr returns the relay server address
func (c *Config) RelayAddr() string {
	return fieldToString(getObjectField(c.ast, "relay_addr"))
}

// SetRelayAddr sets the relay server address
func (c *Config) SetRelayAddr(v string) {
	setStringField(c.ast, "relay_addr", v)
}

// NodeID returns the node identifier
func (c *Config) NodeID() string {
	return fieldToString(getObjectField(c.ast, "node_id"))
}

// SetNodeID sets the node identifier
func (c *Config) SetNodeID(v string) {
	setStringField(c.ast, "node_id", v)
}

// Token returns the authorization token
func (c *Config) Token() string {
	return fieldToString(getObjectField(c.ast, "token"))
}

// SetToken sets the authorization token
func (c *Config) SetToken(v string) {
	setStringField(c.ast, "token", v)
}

// Services returns the services slice
func (c *Config) Services() []Service {
	servicesField := getObjectField(c.ast, "services")
	if servicesField == nil || servicesField.Value == nil {
		return nil
	}
	arr, ok := servicesField.Value.(*hujson.Array)
	if !ok {
		return nil
	}

	var services []Service
	for _, elem := range arr.Elements {
		if elem.Value == nil {
			continue
		}
		obj, ok := elem.Value.(*hujson.Object)
		if !ok {
			continue
		}

		svc := Service{}
		for _, m := range obj.Members {
			name := ""
			if lit, ok := m.Name.Value.(hujson.Literal); ok {
				name = lit.String()
			}

			switch name {
			case "id":
				if lit, ok := m.Value.Value.(hujson.Literal); ok {
					svc.ID = lit.String()
				}
			case "name":
				if lit, ok := m.Value.Value.(hujson.Literal); ok {
					svc.Name = lit.String()
				}
			case "type":
				if lit, ok := m.Value.Value.(hujson.Literal); ok {
					svc.Type = lit.String()
				}
			case "addr":
				if lit, ok := m.Value.Value.(hujson.Literal); ok {
					svc.Addr = lit.String()
				}
			case "port":
				if lit, ok := m.Value.Value.(hujson.Literal); ok {
					svc.Port = int(lit.Int())
				}
			case "path":
				if lit, ok := m.Value.Value.(hujson.Literal); ok {
					svc.Path = lit.String()
				}
			case "auth":
				if authObj, ok := m.Value.Value.(*hujson.Object); ok {
					svc.Auth = &Auth{}
					for _, am := range authObj.Members {
						authName := ""
						if lit, ok := am.Name.Value.(hujson.Literal); ok {
							authName = lit.String()
						}
						switch authName {
						case "type":
							if lit, ok := am.Value.Value.(hujson.Literal); ok {
								svc.Auth.Type = lit.String()
							}
						case "username":
							if lit, ok := am.Value.Value.(hujson.Literal); ok {
								svc.Auth.Username = lit.String()
							}
						case "password":
							if lit, ok := am.Value.Value.(hujson.Literal); ok {
								svc.Auth.Password = lit.String()
							}
						}
					}
				}
			}
		}
		services = append(services, svc)
	}
	return services
}

// AddService adds a new service to the config
func (c *Config) AddService(svc Service) error {
	// Get or create services array
	servicesField := getObjectField(c.ast, "services")
	var servicesArr *hujson.Array
	if servicesField.Value == nil {
		servicesArr = &hujson.Array{}
		c.setServicesArray(servicesArr)
	} else {
		var ok bool
		servicesArr, ok = servicesField.Value.(*hujson.Array)
		if !ok {
			servicesArr = &hujson.Array{}
			c.setServicesArray(servicesArr)
		}
	}

	// Create service object in AST
	svcObj := &hujson.Object{
		Members: []hujson.ObjectMember{
			{Name: hujson.Value{Value: hujson.String("id")}, Value: hujson.Value{Value: hujson.String(svc.ID)}},
			{Name: hujson.Value{Value: hujson.String("name")}, Value: hujson.Value{Value: hujson.String(svc.Name)}},
			{Name: hujson.Value{Value: hujson.String("type")}, Value: hujson.Value{Value: hujson.String(svc.Type)}},
			{Name: hujson.Value{Value: hujson.String("addr")}, Value: hujson.Value{Value: hujson.String(svc.Addr)}},
			{Name: hujson.Value{Value: hujson.String("port")}, Value: hujson.Value{Value: hujson.Int(int64(svc.Port))}},
		},
	}

	if svc.Path != "" {
		svcObj.Members = append(svcObj.Members, hujson.ObjectMember{
			Name: hujson.Value{Value: hujson.String("path")}, Value: hujson.Value{Value: hujson.String(svc.Path)},
		})
	}

	if svc.Auth != nil {
		authObj := &hujson.Object{
			Members: []hujson.ObjectMember{
				{Name: hujson.Value{Value: hujson.String("type")}, Value: hujson.Value{Value: hujson.String(svc.Auth.Type)}},
				{Name: hujson.Value{Value: hujson.String("username")}, Value: hujson.Value{Value: hujson.String(svc.Auth.Username)}},
				{Name: hujson.Value{Value: hujson.String("password")}, Value: hujson.Value{Value: hujson.String(svc.Auth.Password)}},
			},
		}
		svcObj.Members = append(svcObj.Members, hujson.ObjectMember{
			Name: hujson.Value{Value: hujson.String("auth")}, Value: hujson.Value{Value: authObj},
		})
	}

	servicesArr.Elements = append(servicesArr.Elements, hujson.Value{Value: svcObj})
	return nil
}

// setServicesArray sets the services array in the AST
func (c *Config) setServicesArray(arr *hujson.Array) {
	obj, ok := c.ast.Value.(*hujson.Object)
	if !ok {
		return
	}

	// Look for existing services member
	for i, m := range obj.Members {
		if lit, ok := m.Name.Value.(hujson.Literal); ok && lit.String() == "services" {
			m.Value.Value = arr
			obj.Members[i] = m
			return
		}
	}

	// Add new services member
	obj.Members = append(obj.Members, hujson.ObjectMember{
		Name:  hujson.Value{Value: hujson.String("services")},
		Value: hujson.Value{Value: arr},
	})
}

// DeleteService removes a service by index
func (c *Config) DeleteService(index int) error {
	servicesField := getObjectField(c.ast, "services")
	if servicesField.Value == nil {
		return nil
	}
	arr, ok := servicesField.Value.(*hujson.Array)
	if !ok {
		return nil
	}

	if index < 0 || index >= len(arr.Elements) {
		return nil
	}

	// Remove element while preserving other comments
	arr.Elements = append(arr.Elements[:index], arr.Elements[index+1:]...)
	return nil
}

// Save saves the config to disk
func (c *Config) Save() error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}

	c.ast.Format()
	return os.WriteFile(path, c.ast.Pack(), 0600)
}

// ConfigPath returns the path to the config file
func ConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	configDir := filepath.Join(homeDir, ".unblink")
	return filepath.Join(configDir, "config.hujson"), nil
}

// LoadConfig loads the unified config from ~/.unblink/config.hujson
// If the file doesn't exist, creates a new config with a generated node ID
func LoadConfig() (*Config, error) {
	return loadConfigWithDefault(nil)
}

// LoadConfigWithDefault loads the config, using the provided default template if creating new
func LoadConfigWithDefault(defaultConfig []byte) (*Config, error) {
	return loadConfigWithDefault(defaultConfig)
}

// loadConfigWithDefault is the internal implementation
func loadConfigWithDefault(defaultConfig []byte) (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		// File doesn't exist - create new config with generated node ID

		// Create config directory if needed
		configDir := filepath.Dir(path)
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return nil, err
		}

		// Parse default config if provided
		var ast hujson.Value
		if len(defaultConfig) > 0 {
			ast, err = hujson.Parse(defaultConfig)
			if err != nil {
				return nil, err
			}
		} else {
			// Create empty config object
			ast = hujson.Value{Value: &hujson.Object{}}
		}

		config := &Config{ast: &ast}

		// Auto-generate IDs if needed
		if config.NodeID() == "" {
			config.SetNodeID(uuid.New().String())
		}
		for i, svc := range config.Services() {
			if svc.ID == "" {
				// Generate ID and update in AST
				newID := uuid.New().String()
				setServiceID(config.ast, i, newID)
				svc.ID = newID
			}
		}

		// Save with comments
		if err := config.Save(); err != nil {
			return nil, err
		}
		return config, nil
	}

	// Parse existing config
	ast, err := hujson.Parse(data)
	if err != nil {
		return nil, err
	}

	config := &Config{ast: &ast}

	// Auto-generate IDs if needed
	needsSave := false
	if config.NodeID() == "" {
		config.SetNodeID(uuid.New().String())
		needsSave = true
	}

	services := config.Services()
	for i, svc := range services {
		if svc.ID == "" {
			newID := uuid.New().String()
			setServiceID(config.ast, i, newID)
			needsSave = true
		}
	}

	if needsSave {
		config.Save()
	}

	return config, nil
}

// setServiceID sets the ID of a service at a given index in the AST
func setServiceID(ast *hujson.Value, index int, id string) {
	servicesField := getObjectField(ast, "services")
	if servicesField.Value == nil {
		return
	}
	arr, ok := servicesField.Value.(*hujson.Array)
	if !ok {
		return
	}

	if index < 0 || index >= len(arr.Elements) {
		return
	}

	elem := &arr.Elements[index]
	if elem.Value == nil {
		return
	}

	obj, ok := elem.Value.(*hujson.Object)
	if !ok {
		return
	}

	// Look for existing id field or add it
	for i, m := range obj.Members {
		if lit, ok := m.Name.Value.(hujson.Literal); ok && lit.String() == "id" {
			m.Value.Value = hujson.String(id)
			obj.Members[i] = m
			return
		}
	}

	// Add new id field
	obj.Members = append(obj.Members, hujson.ObjectMember{
		Name:  hujson.Value{Value: hujson.String("id")},
		Value: hujson.Value{Value: hujson.String(id)},
	})
}

// SaveConfig saves the unified config (legacy function for backwards compatibility)
// Deprecated: Use config.Save() instead
func SaveConfig(config *Config) error {
	return config.Save()
}
