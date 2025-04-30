# Zanzibar 技術仕様書

このドキュメントは、Google Zanzibar論文（https://storage.googleapis.com/gweb-research2023-media/pubtools/5068.pdf）に基づいて、Zanzibarの技術的な実装詳細を説明するものです。現在の実装状況と、足りていない部分を実装するための具体的な方法について詳細に記述します。

## 目次

- [Zanzibar 技術仕様書](#zanzibar-技術仕様書)
  - [目次](#目次)
  - [1. データモデル](#1-データモデル)
    - [1.1 関係タプル（Relation Tuples）](#11-関係タプルrelation-tuples)
    - [1.2 名前空間設定](#12-名前空間設定)
    - [1.3 Userset Rewrite Rules](#13-userset-rewrite-rules)
  - [2. ストレージレイヤー](#2-ストレージレイヤー)
    - [2.1 抽象化インターフェース](#21-抽象化インターフェース)
    - [2.2 インメモリストレージ](#22-インメモリストレージ)
    - [2.3 永続ストレージ](#23-永続ストレージ)
    - [2.4 変更ログ](#24-変更ログ)
  - [3. 一貫性モデル](#3-一貫性モデル)
    - [3.1 外部一貫性](#31-外部一貫性)
    - [3.2 スナップショット読み取り](#32-スナップショット読み取り)
    - [3.3 Zookieプロトコル](#33-zookieプロトコル)
  - [4. API実装](#4-api実装)
    - [4.1 Check API](#41-check-api)
    - [4.2 Read API](#42-read-api)
    - [4.3 Write API](#43-write-api)
    - [4.4 Expand API](#44-expand-api)
    - [4.5 Watch API](#45-watch-api)
  - [5. 評価エンジン](#5-評価エンジン)
    - [5.1 チェック評価](#51-チェック評価)

## 1. データモデル

### 1.1 関係タプル（Relation Tuples）

関係タプルは、Zanzibarのデータモデルの基本単位です。以下の構造で表現されます：

```go
// Relationship represents a relationship between a resource and a subject
type Relationship struct {
    Resource    string    `json:"resource"`    // 形式: namespace:object_id
    Relation    string    `json:"relation"`    // リレーション名
    Subject     string    `json:"subject"`     // ユーザーIDまたはuserset
    ZookieToken string    `json:"zookie_token,omitempty"` // 一貫性トークン
    UpdatedAt   time.Time `json:"updated_at"`  // 更新時刻
}
```

関係タプルのテキスト表現は以下の形式です：

```
namespace:object_id#relation@subject
```

ここで、`subject`は以下のいずれかです：
- ユーザーID: `user_id`
- ユーザーセット: `namespace:object_id#relation`

例：
```
document:doc1#owner@user1
document:doc1#viewer@group:eng#member
```

### 1.2 名前空間設定

名前空間設定は、リソースタイプとそのリレーションを定義します：

```go
// Definition represents a resource type definition
type Definition struct {
    Type        string                `json:"type"`        // 名前空間名
    Relations   map[string]Relation   `json:"relations"`   // リレーション定義
    Permissions map[string]Permission `json:"permissions"` // パーミッション定義
}

// Relation defines a relationship between resources
type Relation struct {
    Subjects       []Subject       `json:"subjects"`       // 許可されるサブジェクトタイプ
    UsersetRewrite *UsersetRewrite `json:"userset_rewrite,omitempty"` // リライトルール
}

// Subject defines a subject that can be in a relation
type Subject struct {
    Type     string `json:"type"`     // サブジェクトタイプ
    Relation string `json:"relation,omitempty"` // オプションのリレーション
}

// Permission defines a permission expression
type Permission struct {
    Expression string `json:"expression"` // パーミッション式
}
```

名前空間設定の例：

```json
{
  "document": {
    "relations": {
      "owner": {
        "subjects": [{"type": "user"}]
      },
      "editor": {
        "subjects": [{"type": "user"}],
        "userset_rewrite": {...}
      },
      "viewer": {
        "subjects": [
          {"type": "user"},
          {"type": "group", "relation": "member"}
        ],
        "userset_rewrite": {...}
      },
      "parent": {
        "subjects": [{"type": "folder"}]
      }
    },
    "permissions": {
      "view": {"expression": "owner | editor | viewer"},
      "edit": {"expression": "owner | editor"},
      "delete": {"expression": "owner"}
    }
  }
}
```

### 1.3 Userset Rewrite Rules

Userset Rewrite Rulesは、オブジェクトの実効的なACLを定義するためのルールです：

```go
// UsersetRewrite defines a rule for computing a userset
type UsersetRewrite struct {
    Type            UsersetRewriteType `json:"type"`
    ComputedUserset *ComputedUserset   `json:"computed_userset,omitempty"`
    TupleToUserset  *TupleToUserset    `json:"tuple_to_userset,omitempty"`
    Children        []*UsersetRewrite  `json:"children,omitempty"`
}

// UsersetRewriteType defines the type of userset rewrite rule
type UsersetRewriteType string

const (
    UsersetRewriteThis          UsersetRewriteType = "this"
    UsersetRewriteComputedUserset UsersetRewriteType = "computed_userset"
    UsersetRewriteTupleToUserset  UsersetRewriteType = "tuple_to_userset"
    UsersetRewriteUnion           UsersetRewriteType = "union"
    UsersetRewriteIntersection    UsersetRewriteType = "intersection"
    UsersetRewriteExclusion       UsersetRewriteType = "exclusion"
)

// ComputedUserset represents a userset computed from another relation on the same object
type ComputedUserset struct {
    Relation string `json:"relation"`
}

// TupleToUserset represents a userset computed from a relation on another object
type TupleToUserset struct {
    Tupleset        Tupleset        `json:"tupleset"`
    ComputedUserset ComputedUserset `json:"computed_userset"`
}

// Tupleset defines a set of tuples to look up
type Tupleset struct {
    Relation string `json:"relation"`
}
```

Userset Rewrite Rulesの例（ドキュメントのビューア権限）：

```json
{
  "userset_rewrite": {
    "union": {
      "child": [
        {"_this": {}},
        {"computed_userset": {"relation": "editor"}},
        {
          "tuple_to_userset": {
            "tupleset": {"relation": "parent"},
            "computed_userset": {
              "object": "$TUPLE_USERSET_OBJECT",
              "relation": "viewer"
            }
          }
        }
      ]
    }
  }
}
```

この例では：
- 直接ビューアとして指定されたユーザー（`this`）
- エディタ権限を持つユーザー（`computed_userset`）
- 親フォルダのビューア権限を持つユーザー（`tuple_to_userset`）
がドキュメントのビューア権限を持ちます。

## 2. ストレージレイヤー

### 2.1 抽象化インターフェース

ストレージレイヤーは、異なるバックエンドをサポートするための抽象化インターフェースを提供します：

```go
// Storage defines the interface for storage backends
type Storage interface {
    // Read operations
    ReadRelationship(ctx context.Context, resource, relation, subject string) (*Relationship, error)
    ReadResourceRelationships(ctx context.Context, resource string) ([]Relationship, error)
    ReadRelationships(ctx context.Context, filter RelationshipFilter, snapshot Snapshot) ([]Relationship, error)
    
    // Write operations
    WriteRelationship(ctx context.Context, relationship *Relationship) (string, error)
    DeleteRelationship(ctx context.Context, resource, relation, subject string) error
    
    // Consistency operations
    GetLatestSnapshot(ctx context.Context) (Snapshot, error)
    GetSnapshotAtLeast(ctx context.Context, minSnapshot Snapshot) (Snapshot, error)
    
    // Watch operations
    WatchChanges(ctx context.Context, startSnapshot Snapshot) (<-chan ChangeEvent, error)
    
    // Namespace operations
    ReadNamespaceConfig(ctx context.Context, namespace string, snapshot Snapshot) (*Definition, error)
    WriteNamespaceConfig(ctx context.Context, namespace string, definition *Definition) error
}

// RelationshipFilter defines filters for reading relationships
type RelationshipFilter struct {
    Resource    string
    Relation    string
    Subject     string
    Namespace   string
}

// Snapshot represents a point-in-time view of the storage
type Snapshot struct {
    Timestamp time.Time
    Token     string // zookie
}

// ChangeEvent represents a change in the storage
type ChangeEvent struct {
    Type         ChangeType
    Relationship Relationship
    Timestamp    time.Time
}

type ChangeType string

const (
    ChangeTypeCreate ChangeType = "create"
    ChangeTypeUpdate ChangeType = "update"
    ChangeTypeDelete ChangeType = "delete"
)
```

### 2.2 インメモリストレージ

開発とテスト用のインメモリストレージ実装：

```go
// MemoryStorage implements the Storage interface using in-memory data structures
type MemoryStorage struct {
    relationships []Relationship
    configs       map[string]*Definition
    mu            sync.RWMutex
    changeNumber  int64
    watchers      map[string][]chan ChangeEvent
}

// NewMemoryStorage creates a new memory storage
func NewMemoryStorage() *MemoryStorage {
    return &MemoryStorage{
        relationships: make([]Relationship, 0),
        configs:       make(map[string]*Definition),
        changeNumber:  1,
        watchers:      make(map[string][]chan ChangeEvent),
    }
}
```

### 2.3 永続ストレージ

永続ストレージの実装（PostgreSQL例）：

```go
// PostgresStorage implements the Storage interface using PostgreSQL
type PostgresStorage struct {
    db *sql.DB
}

// NewPostgresStorage creates a new PostgreSQL storage
func NewPostgresStorage(connStr string) (*PostgresStorage, error) {
    db, err := sql.Open("postgres", connStr)
    if err != nil {
        return nil, err
    }
    
    // Initialize schema
    if err := initSchema(db); err != nil {
        return nil, err
    }
    
    return &PostgresStorage{db: db}, nil
}

func initSchema(db *sql.DB) error {
    // Create relationships table
    _, err := db.Exec(`
        CREATE TABLE IF NOT EXISTS relationships (
            resource TEXT NOT NULL,
            relation TEXT NOT NULL,
            subject TEXT NOT NULL,
            zookie_token TEXT,
            updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
            PRIMARY KEY (resource, relation, subject)
        )
    `)
    if err != nil {
        return err
    }
    
    // Create namespace configs table
    _, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS namespace_configs (
            namespace TEXT PRIMARY KEY,
            definition JSONB NOT NULL,
            updated_at TIMESTAMP WITH TIME ZONE NOT NULL
        )
    `)
    if err != nil {
        return err
    }
    
    // Create changelog table
    _, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS changelog (
            id SERIAL PRIMARY KEY,
            change_type TEXT NOT NULL,
            resource TEXT NOT NULL,
            relation TEXT NOT NULL,
            subject TEXT NOT NULL,
            timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
            zookie_token TEXT
        )
    `)
    return err
}
```

### 2.4 変更ログ

変更ログは、関係タプルの変更を時系列で記録するためのコンポーネントです：

```go
// ChangeLog represents a log of changes to relationships
type ChangeLog struct {
    storage Storage
}

// NewChangeLog creates a new change log
func NewChangeLog(storage Storage) *ChangeLog {
    return &ChangeLog{
        storage: storage,
    }
}

// LogChange logs a change to a relationship
func (c *ChangeLog) LogChange(ctx context.Context, changeType ChangeType, relationship *Relationship) error {
    // Implementation depends on the storage backend
    return nil
}

// GetChanges gets changes since a specific snapshot
func (c *ChangeLog) GetChanges(ctx context.Context, startSnapshot Snapshot) (<-chan ChangeEvent, error) {
    return c.storage.WatchChanges(ctx, startSnapshot)
}
```

## 3. 一貫性モデル

### 3.1 外部一貫性

外部一貫性は、因果関係のある更新の順序を尊重するために重要です：

```go
// ConsistencyManager manages consistency guarantees
type ConsistencyManager struct {
    storage Storage
}

// NewConsistencyManager creates a new consistency manager
func NewConsistencyManager(storage Storage) *ConsistencyManager {
    return &ConsistencyManager{
        storage: storage,
    }
}

// GetLatestSnapshot gets the latest snapshot
func (c *ConsistencyManager) GetLatestSnapshot(ctx context.Context) (Snapshot, error) {
    return c.storage.GetLatestSnapshot(ctx)
}

// GetSnapshotAtLeast gets a snapshot at least as fresh as the specified snapshot
func (c *ConsistencyManager) GetSnapshotAtLeast(ctx context.Context, minSnapshot Snapshot) (Snapshot, error) {
    return c.storage.GetSnapshotAtLeast(ctx, minSnapshot)
}
```

### 3.2 スナップショット読み取り

スナップショット読み取りは、一貫性のあるデータビューを提供します：

```go
// SnapshotReader reads data at a specific snapshot
type SnapshotReader struct {
    storage Storage
}

// NewSnapshotReader creates a new snapshot reader
func NewSnapshotReader(storage Storage) *SnapshotReader {
    return &SnapshotReader{
        storage: storage,
    }
}

// ReadRelationships reads relationships at a specific snapshot
func (r *SnapshotReader) ReadRelationships(ctx context.Context, filter RelationshipFilter, snapshot Snapshot) ([]Relationship, error) {
    return r.storage.ReadRelationships(ctx, filter, snapshot)
}
```

### 3.3 Zookieプロトコル

Zookieプロトコルは、クライアントが一貫性のある読み取りを行うためのメカニズムです：

```go
// ZookieManager manages zookie tokens
type ZookieManager struct {
    consistencyManager *ConsistencyManager
}

// NewZookieManager creates a new zookie manager
func NewZookieManager(consistencyManager *ConsistencyManager) *ZookieManager {
    return &ZookieManager{
        consistencyManager: consistencyManager,
    }
}

// GenerateZookie generates a new zookie token
func (z *ZookieManager) GenerateZookie(ctx context.Context) (string, error) {
    snapshot, err := z.consistencyManager.GetLatestSnapshot(ctx)
    if err != nil {
        return "", err
    }
    
    // Encode the snapshot timestamp in the zookie
    return encodeZookie(snapshot), nil
}

// ParseZookie parses a zookie token
func (z *ZookieManager) ParseZookie(zookie string) (Snapshot, error) {
    return decodeZookie(zookie)
}

// encodeZookie encodes a snapshot as a zookie token
func encodeZookie(snapshot Snapshot) string {
    // In a real implementation, this would use a secure encoding
    return fmt.Sprintf("zk_%d", snapshot.Timestamp.UnixNano())
}

// decodeZookie decodes a zookie token to a snapshot
func decodeZookie(zookie string) (Snapshot, error) {
    // In a real implementation, this would validate and decode the token
    if !strings.HasPrefix(zookie, "zk_") {
        return Snapshot{}, fmt.Errorf("invalid zookie format")
    }
    
    timestampStr := strings.TrimPrefix(zookie, "zk_")
    timestampNano, err := strconv.ParseInt(timestampStr, 10, 64)
    if err != nil {
        return Snapshot{}, err
    }
    
    return Snapshot{
        Timestamp: time.Unix(0, timestampNano),
        Token:     zookie,
    }, nil
}
```

## 4. API実装

### 4.1 Check API

Check APIは、サブジェクトがリソースに対して特定の許可を持っているかどうかを確認します：

```go
// CheckRequest represents a check request
type CheckRequest struct {
    Subject     string `json:"subject"`
    Resource    string `json:"resource"`
    Relation    string `json:"relation"`
    ZookieToken string `json:"zookie_token,omitempty"`
}

// CheckResponse represents a check response
type CheckResponse struct {
    Allowed     bool   `json:"allowed"`
    Reason      string `json:"reason,omitempty"`
    ZookieToken string `json:"zookie_token,omitempty"`
}

// Check checks if a subject has a relation to a resource
func (s *Server) Check(ctx context.Context, req *CheckRequest) (*CheckResponse, error) {
    // Parse the zookie token
    var snapshot Snapshot
    var err error
    if req.ZookieToken != "" {
        snapshot, err = s.zookieManager.ParseZookie(req.ZookieToken)
        if err != nil {
            return nil, err
        }
    } else {
        // Use a recent snapshot
        snapshot, err = s.consistencyManager.GetLatestSnapshot(ctx)
        if err != nil {
            return nil, err
        }
    }
    
    // Evaluate the check
    allowed, reason, err := s.evaluator.EvaluateCheck(ctx, req.Subject, req.Resource, req.Relation, snapshot)
    if err != nil {
        return nil, err
    }
    
    return &CheckResponse{
        Allowed:     allowed,
        Reason:      reason,
        ZookieToken: snapshot.Token,
    }, nil
}
```

### 4.2 Read API

Read APIは、関係タプルを読み取ります：

```go
// ReadRequest represents a read request
type ReadRequest struct {
    Resource    string `json:"resource,omitempty"`
    Relation    string `json:"relation,omitempty"`
    Subject     string `json:"subject,omitempty"`
    ZookieToken string `json:"zookie_token,omitempty"`
}

// ReadResponse represents a read response
type ReadResponse struct {
    Relationships []Relationship `json:"relationships"`
    ZookieToken   string         `json:"zookie_token,omitempty"`
}

// Read reads relationships
func (s *Server) Read(ctx context.Context, req *ReadRequest) (*ReadResponse, error) {
    // Parse the zookie token
    var snapshot Snapshot
    var err error
    if req.ZookieToken != "" {
        snapshot, err = s.zookieManager.ParseZookie(req.ZookieToken)
        if err != nil {
            return nil, err
        }
    } else {
        // Use a recent snapshot
        snapshot, err = s.consistencyManager.GetLatestSnapshot(ctx)
        if err != nil {
            return nil, err
        }
    }
    
    // Create a filter
    filter := RelationshipFilter{
        Resource: req.Resource,
        Relation: req.Relation,
        Subject:  req.Subject,
    }
    
    // Read relationships
    relationships, err := s.snapshotReader.ReadRelationships(ctx, filter, snapshot)
    if err != nil {
        return nil, err
    }
    
    return &ReadResponse{
        Relationships: relationships,
        ZookieToken:   snapshot.Token,
    }, nil
}
```

### 4.3 Write API

Write APIは、関係タプルを追加または削除します：

```go
// WriteRequest represents a write request
type WriteRequest struct {
    Relationships []Relationship `json:"relationships"`
    Delete        bool           `json:"delete,omitempty"`
}

// WriteResponse represents a write response
type WriteResponse struct {
    ZookieToken string `json:"zookie_token,omitempty"`
}

// Write writes relationships
func (s *Server) Write(ctx context.Context, req *WriteRequest) (*WriteResponse, error) {
    var zookieToken string
    var err error
    
    for _, relationship := range req.Relationships {
        if req.Delete {
            err = s.storage.DeleteRelationship(ctx, relationship.Resource, relationship.Relation, relationship.Subject)
        } else {
            zookieToken, err = s.storage.WriteRelationship(ctx, &relationship)
        }
        
        if err != nil {
            return nil, err
        }
    }
    
    // If no zookie token was generated, generate one
    if zookieToken == "" {
        zookieToken, err = s.zookieManager.GenerateZookie(ctx)
        if err != nil {
            return nil, err
        }
    }
    
    return &WriteResponse{
        ZookieToken: zookieToken,
    }, nil
}
```

### 4.4 Expand API

Expand APIは、特定のオブジェクトとリレーションに対する実効的なユーザーセットを返します：

```go
// ExpandRequest represents an expand request
type ExpandRequest struct {
    Resource    string `json:"resource"`
    Relation    string `json:"relation"`
    ZookieToken string `json:"zookie_token,omitempty"`
}

// ExpandResponse represents an expand response
type ExpandResponse struct {
    Tree        *UsersetTree `json:"tree"`
    ZookieToken string       `json:"zookie_token,omitempty"`
}

// UsersetTree represents a userset tree
type UsersetTree struct {
    Type     UsersetTreeType `json:"type"`
    Subject  string          `json:"subject,omitempty"`
    Relation string          `json:"relation,omitempty"`
    Resource string          `json:"resource,omitempty"`
    Children []*UsersetTree  `json:"children,omitempty"`
}

type UsersetTreeType string

const (
    UsersetTreeTypeLeaf        UsersetTreeType = "leaf"
    UsersetTreeTypeUnion       UsersetTreeType = "union"
    UsersetTreeTypeIntersection UsersetTreeType = "intersection"
    UsersetTreeTypeExclusion   UsersetTreeType = "exclusion"
)

// Expand expands a userset
func (s *Server) Expand(ctx context.Context, req *ExpandRequest) (*ExpandResponse, error) {
    // Parse the zookie token
    var snapshot Snapshot
    var err error
    if req.ZookieToken != "" {
        snapshot, err = s.zookieManager.ParseZookie(req.ZookieToken)
        if err != nil {
            return nil, err
        }
    } else {
        // Use a recent snapshot
        snapshot, err = s.consistencyManager.GetLatestSnapshot(ctx)
        if err != nil {
            return nil, err
        }
    }
    
    // Expand the userset
    tree, err := s.evaluator.ExpandUserset(ctx, req.Resource, req.Relation, snapshot)
    if err != nil {
        return nil, err
    }
    
    return &ExpandResponse{
        Tree:        tree,
        ZookieToken: snapshot.Token,
    }, nil
}
```

### 4.5 Watch API

Watch APIは、関係タプルの更新を監視します：

```go
// WatchRequest represents a watch request
type WatchRequest struct {
    Namespace   string `json:"namespace,omitempty"`
    ZookieToken string `json:"zookie_token,omitempty"`
}

// WatchResponse represents a watch response
type WatchResponse struct {
    Events      []ChangeEvent `json:"events"`
    ZookieToken string        `json:"zookie_token,omitempty"`
}

// Watch watches for changes
func (s *Server) Watch(ctx context.Context, req *WatchRequest) (<-chan *WatchResponse, error) {
    // Parse the zookie token
    var snapshot Snapshot
    var err error
    if req.ZookieToken != "" {
        snapshot, err = s.zookieManager.ParseZookie(req.ZookieToken)
        if err != nil {
            return nil, err
        }
    } else {
        // Use a recent snapshot
        snapshot, err = s.consistencyManager.GetLatestSnapshot(ctx)
        if err != nil {
            return nil, err
        }
    }
    
    // Watch for changes
    eventCh, err := s.changeLog.GetChanges(ctx, snapshot)
    if err != nil {
        return nil, err
    }
    
    // Create a response channel
    responseCh := make(chan *WatchResponse)
    
    // Start a goroutine to process events
    go func() {
        defer close(responseCh)
        
        var events []ChangeEvent
        ticker := time.NewTicker(5 * time.Second)
        defer ticker.Stop()
        
        for {
            select {
            case event, ok := <-eventCh:
                if !ok {
                    // Channel closed
                    if len(events) > 0 {
                        responseCh <- &WatchResponse{
                            Events:      events,
                            ZookieToken: encodeZookie(Snapshot{Timestamp: events[len(events)-1].Timestamp}),
                        }
                    }
                    return
                }
                
                events = append(events, event)
                
                // If we have enough events, send them
                if len(events) >= 100 {
                    responseCh <- &WatchResponse{
                        Events:      events,
                        ZookieToken: encodeZookie(Snapshot{Timestamp: events[len(events)-1].Timestamp}),
                    }
                    events = nil
                }
                
            case <-ticker.C:
                // Send heartbeat
                if len(events) > 0 {
                    responseCh <- &WatchResponse{
                        Events:      events,
                        ZookieToken: encodeZookie(Snapshot{Timestamp: events[len(events)-1].Timestamp}),
                    }
                    events = nil
                } else {
                    // Send empty heartbeat
                    latestSnapshot, err := s.consistencyManager.GetLatestSnapshot(ctx)
                    if err == nil {
                        responseCh <- &WatchResponse{
                            Events:      nil,
                            ZookieToken: latestSnapshot.Token,
                        }
                    }
                }
                
            case <-ctx.Done():
                // Context cancelled
                return
            }
        }
    }()
    
    return responseCh, nil
}
```

## 5. 評価エンジン

### 5.1 チェック評価

チェック評価は、サブジェクトがリソースに対して特定のリレーションを持っているかどうかを評価します：

```go
// Evaluator evaluates access control checks
type Evaluator struct {
    storage         Storage
    snapshotReader  *SnapshotReader
    usersetEvaluator *UsersetEvaluator
}

// NewEvaluator creates a new evaluator
func NewEvaluator(storage Storage, snapshotReader *SnapshotReader) *Evaluator {
    usersetEvaluator := NewUsersetEvaluator(storage, snapshotReader)
    return &Evaluator{
        storage:         storage,
        snapshotReader:  snapshotReader,
        usersetEvaluator: usersetEvaluator,
    }
}

// EvaluateCheck evaluates if a subject has a relation
