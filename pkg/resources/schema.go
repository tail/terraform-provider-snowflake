package resources

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/helper/validation"
	"github.com/pkg/errors"

	"github.com/chanzuckerberg/terraform-provider-snowflake/pkg/snowflake"
)

var schemaSchema = map[string]*schema.Schema{
	"name": &schema.Schema{
		Type:        schema.TypeString,
		Required:    true,
		Description: "Specifies the identifier for the schema; must be unique for the database in which the schema is created.",
		ForceNew:    true,
	},
	"database": &schema.Schema{
		Type:        schema.TypeString,
		Required:    true,
		Description: "The database in which to create the schema.",
		ForceNew:    true,
	},
	"comment": &schema.Schema{
		Type:        schema.TypeString,
		Optional:    true,
		Description: "Specifies a comment for the schema.",
	},
	"is_transient": &schema.Schema{
		Type:        schema.TypeBool,
		Optional:    true,
		Default:     false,
		Description: "Specifies a schema as transient. Transient schemas do not have a Fail-safe period so they do not incur additional storage costs once they leave Time Travel; however, this means they are also not protected by Fail-safe in the event of a data loss.",
		ForceNew:    true,
	},
	"is_managed": &schema.Schema{
		Type:        schema.TypeBool,
		Optional:    true,
		Default:     false,
		Description: "Specifies a managed schema. Managed access schemas centralize privilege management with the schema owner.",
	},
	"data_retention_days": &schema.Schema{
		Type:         schema.TypeInt,
		Optional:     true,
		Default:      1,
		Description:  "Specifies the number of days for which Time Travel actions (CLONE and UNDROP) can be performed on the schema, as well as specifying the default Time Travel retention time for all tables created in the schema.",
		ValidateFunc: validation.IntBetween(0, 90),
	},
}

// Schema returns a pointer to the resource representing a schema
func Schema() *schema.Resource {
	return &schema.Resource{
		Create: CreateSchema,
		Read:   ReadSchema,
		Update: UpdateSchema,
		Delete: DeleteSchema,
		Exists: SchemaExists,

		Schema: schemaSchema,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
	}
}

// CreateSchema implements schema.CreateFunc
func CreateSchema(data *schema.ResourceData, meta interface{}) error {
	db := meta.(*sql.DB)
	name := data.Get("name").(string)
	database := data.Get("database").(string)

	builder := snowflake.Schema(name).WithDB(database)

	// Set optionals
	if v, ok := data.GetOk("comment"); ok {
		builder.WithComment(v.(string))
	}

	if v, ok := data.GetOk("is_transient"); ok && v.(bool) {
		builder.Transient()
	}

	if v, ok := data.GetOk("is_managed"); ok && v.(bool) {
		builder.Managed()
	}

	if v, ok := data.GetOk("data_retention_days"); ok {
		builder.WithDataRetentionDays(v.(int))
	}

	q := builder.Create()

	err := DBExec(db, q)
	if err != nil {
		return errors.Wrapf(err, "error creating schema %v", name)
	}

	// ID format is <database>|<schema> - please don't use a pipe in your names!
	data.SetId(fmt.Sprintf("%v|%v", database, name))

	return ReadSchema(data, meta)
}

// ReadSchema implements schema.ReadFunc
func ReadSchema(data *schema.ResourceData, meta interface{}) error {
	db := meta.(*sql.DB)
	dbName, schema, err := splitSchemaID(data.Id())
	if err != nil {
		return err
	}

	q := snowflake.Schema(schema).WithDB(dbName).Show()
	row := db.QueryRow(q)
	var createdOn, name, isDefault, isCurrent, databaseName, owner, comment, options sql.NullString
	var retentionTime sql.NullInt64
	err = row.Scan(&createdOn, &name, &isDefault, &isCurrent, &databaseName, &owner, &comment, &options, &retentionTime)
	if err != nil {
		return err
	}

	// TODO turn this into a loop after we switch to scaning in a struct
	err = data.Set("name", name.String)
	if err != nil {
		return err
	}

	err = data.Set("database", databaseName.String)
	if err != nil {
		return err
	}

	err = data.Set("comment", comment.String)
	if err != nil {
		return err
	}

	err = data.Set("data_retention_days", retentionTime.Int64)
	if err != nil {
		return err
	}

	// reset the options before reading back from the DB
	err = data.Set("is_transient", false)
	if err != nil {
		return err
	}

	err = data.Set("is_managed", false)
	if err != nil {
		return err
	}

	if opts := options.String; opts != "" {
		for _, opt := range strings.Split(opts, ", ") {
			switch opt {
			case "TRANSIENT":
				err = data.Set("is_transient", true)
				if err != nil {
					return err
				}
			case "MANAGED":
				err = data.Set("is_managed", true)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// UpdateSchema implements schema.UpdateFunc
func UpdateSchema(data *schema.ResourceData, meta interface{}) error {
	// https://www.terraform.io/docs/extend/writing-custom-providers.html#error-handling-amp-partial-state
	data.Partial(true)

	dbName, schema, err := splitSchemaID(data.Id())
	if err != nil {
		return err
	}

	builder := snowflake.Schema(schema).WithDB(dbName)

	db := meta.(*sql.DB)
	if data.HasChange("comment") {
		_, comment := data.GetChange("comment")
		q := builder.ChangeComment(comment.(string))
		err := DBExec(db, q)
		if err != nil {
			return errors.Wrapf(err, "error updating schema comment on %v", data.Id())
		}

		data.SetPartial("comment")
	}

	if data.HasChange("is_managed") {
		_, managed := data.GetChange("is_managed")
		var q string
		if managed.(bool) {
			q = builder.Manage()
		} else {
			q = builder.Unmanage()
		}

		err := DBExec(db, q)
		if err != nil {
			return errors.Wrapf(err, "error changing management state on %v", data.Id())
		}

		data.SetPartial("is_managed")
	}

	data.Partial(false)
	if data.HasChange("data_retention_days") {
		_, days := data.GetChange("data_retention_days")

		q := builder.ChangeDataRetentionDays(days.(int))
		err := DBExec(db, q)
		if err != nil {
			return errors.Wrapf(err, "error updating data retention days on %v", data.Id())
		}
	}

	return ReadSchema(data, meta)
}

// DeleteSchema implements schema.DeleteFunc
func DeleteSchema(data *schema.ResourceData, meta interface{}) error {
	db := meta.(*sql.DB)
	dbName, schema, err := splitSchemaID(data.Id())
	if err != nil {
		return err
	}

	q := snowflake.Schema(schema).WithDB(dbName).Drop()

	err = DBExec(db, q)
	if err != nil {
		return errors.Wrapf(err, "error deleting schema %v", data.Id())
	}

	data.SetId("")

	return nil
}

// SchemaExists implements schema.ExistsFunc
func SchemaExists(data *schema.ResourceData, meta interface{}) (bool, error) {
	db := meta.(*sql.DB)
	dbName, schema, err := splitSchemaID(data.Id())
	if err != nil {
		return false, err
	}

	q := snowflake.Schema(schema).WithDB(dbName).Show()
	rows, err := db.Query(q)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	if rows.Next() {
		return true, nil
	}

	return false, nil
}

// splitSchemaID takes the <database_name>|<schema_name> ID and returns the
// database name and schema name.
func splitSchemaID(v string) (string, string, error) {
	arr := strings.Split(v, "|")
	if len(arr) != 2 {
		return "", "", fmt.Errorf("ID %v is invalid", v)
	}

	return arr[0], arr[1], nil
}
