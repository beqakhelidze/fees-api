package fees

TemporalServer: [
	if #Meta.Environment.Cloud == "local" { "localhost:7233" },
	"localhost:7233",
][0]
