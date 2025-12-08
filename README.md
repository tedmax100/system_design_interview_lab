----

## Local conntect to PostgreSQL

```bash
kubectl port-forward svc/postgresql 5432:5432 -n leaderboard
```

DataGrip 連線設定：
```
  | 設定項      | 值         |
  |----------|-------------|
  | Host     | localhost   |
  | Port     | 5432        |
  | Database | leaderboard |
  | User     | postgres    |
  | Password | postgres123 |
```