# Connecting to Databases with DataGrip

This guide covers connecting to all project databases from JetBrains DataGrip while the infrastructure is running locally via `docker compose`.

## Prerequisites

- Infrastructure running: `make infra` or `docker compose -f infrastructure/docker-compose.yml up -d`
- DataGrip installed

---

## PostgreSQL Databases

All Postgres instances use the same credentials with different ports and database names.

| Service        | Host      | Port   | Database        | User       | Password   |
|----------------|-----------|--------|-----------------|------------|------------|
| comments-db    | localhost | 5433   | comments        | postgres   | postgres   |
| likes-db       | localhost | 5434   | likes           | postgres   | postgres   |
| users-db       | localhost | 5436   | users           | postgres   | postgres   |
| notifications-db | localhost | 5437 | notifications   | postgres   | postgres   |

### Steps

1. **File → New → Data Source → PostgreSQL**
2. Fill in the connection details from the table above
3. Set **Authentication** to `User & Password`
4. Click **Test Connection** to verify
5. Click **OK**

### JDBC URL format

```
jdbc:postgresql://localhost:<port>/<database>
```

Example for comments:
```
jdbc:postgresql://localhost:5433/comments
```

---

## ScyllaDB (posts-db)

| Field     | Value         |
|-----------|---------------|
| Host      | localhost     |
| Port      | 9042          |
| Keyspace  | posts         |

### Steps

1. **File → New → Data Source → Apache Cassandra**
   - ScyllaDB is CQL-compatible, so use the Cassandra driver
2. Set **Host** to `localhost`, **Port** to `9042`
3. Leave authentication empty (no auth configured)
4. Set the default keyspace to `posts` (optional)
5. Click **Test Connection**, then **OK**

> Note: You may need to download the Cassandra JDBC driver when prompted by DataGrip.

---

## ClickHouse (event store)

| Field     | Value         |
|-----------|---------------|
| Host      | localhost     |
| HTTP Port | 8123          |
| TCP Port  | 9009          |
| User      | default       |
| Password  | clickhouse    |
| Database  | default       |

### Steps

1. **File → New → Data Source → ClickHouse**
2. Set **Host** to `localhost`, **Port** to `8123`
3. Set **User** to `default`, leave **Password** empty
4. Click **Test Connection**, then **OK**

### JDBC URL format

```
jdbc:clickhouse://localhost:8123/default
```

### Troubleshooting

If you get `Unknown and unmapped config properties: [databaseTerm, session_id]`:

1. Go to the **Advanced** tab in data source properties
2. Clear the values for `databaseTerm` and `session_id`
3. Retry the connection

Alternatively, update the ClickHouse JDBC driver via **Drivers** tab → ClickHouse → download latest version.

---

## Tips

- **Name your data sources** clearly (e.g., `[DSN] comments-db`, `[DSN] posts-scylla`) to avoid confusion between similar Postgres instances.
- **Color-code** each data source in DataGrip (right-click → Change Color) to visually distinguish environments.
- **Schemas tab**: After connecting to Postgres, go to the Schemas tab in data source properties to select only the `public` schema — keeps the navigator clean.
- **SSH Tunnel**: If connecting to remote/deployed databases, configure an SSH tunnel under the SSH/SSL tab in the data source properties.
