package ent

import "entgo.io/ent/dialect"

// Driver returns the underlying driver of the client.
// This method is used to access the sql.DB for raw SQL operations.
func (c *Client) Driver() dialect.Driver {
	return c.driver
}
