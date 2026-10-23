---
name: gorm-db-patterns
description: >-
  Use this skill when modifying database schemas, writing GORM queries, or optimizing database operations in the Zahra Go backend. Supports SQLite and PostgreSQL dual setups.
---

# GORM & Database Patterns for Zahra Backend

## Dual Database Support

The project supports both SQLite (default local) and PostgreSQL (production):
- SQLite driver: `github.com/glebarez/sqlite` (pure Go, CGO-free) or `gorm.io/driver/sqlite`.
- PostgreSQL driver: `gorm.io/driver/postgres`.

## Query Guidelines

1. **Preload Relationships**: Explicitly preload related entities when needed to avoid unintended queries:
   ```go
   db.Preload("Category").Preload("Account").Find(&transactions)
   ```
2. **Transaction Handling**: For operations touching multiple tables (e.g. updating balance when transaction is created):
   ```go
   err := db.Transaction(func(tx *gorm.DB) error {
       // execute operations with tx
       return nil
   })
   ```
3. **Soft Deletes**: Use `gorm.DeletedAt` where data retention and auditability are required.
4. **Indexes**: Ensure foreign keys (`account_id`, `category_id`) and frequent filter fields (`date`, `type`) have indexes defined in GORM struct tags.
