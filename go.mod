module github.com/chanzuckerberg/terraform-provider-snowflake

go 1.12

require (
	github.com/DATA-DOG/go-sqlmock v1.3.3
	github.com/ExpansiveWorlds/instrumentedsql v0.0.0-20171218214018-45abb4b1947d
	github.com/Pallinder/go-randomdata v1.2.0
	github.com/SermoDigital/jose v0.0.0-20161205224733-f6df55f235c2 // indirect
	github.com/hashicorp/terraform v0.12.7
	github.com/jmoiron/sqlx v1.2.0
	github.com/olekukonko/tablewriter v0.0.1
	github.com/opentracing/opentracing-go v1.1.0 // indirect
	github.com/pkg/browser v0.0.0-20180916011732-0a3d74bf9ce4 // indirect
	github.com/pkg/errors v0.8.1
	github.com/snowflakedb/gosnowflake v1.2.0
	github.com/stretchr/testify v1.4.0
)

// TODO: when https://github.com/hashicorp/terraform/issues/22664 gets resolved, remove this line:
replace git.apache.org/thrift.git => github.com/apache/thrift v0.0.0-20180902110319-2566ecd5d999
