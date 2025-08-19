package postgres

type DatabaseInstance struct {
	StatefulsetName      string
	ServiceName          string
	Namespace            string
	Username             string
	Password             string
	ImageName            string
	UserLocation         string
	UserLocationType     string
	UserKey              string
	PasswordLocation     string
	PasswordLocationType string
	PasswordKey          string
}
