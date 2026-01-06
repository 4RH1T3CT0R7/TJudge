package pagination

// Edge представляет элемент в пагинированном списке с курсором
type Edge[T any] struct {
	Node   T      `json:"node"`
	Cursor string `json:"cursor"`
}

// Connection представляет пагинированный ответ в формате GraphQL Relay
type Connection[T any] struct {
	Edges    []*Edge[T] `json:"edges"`
	PageInfo PageInfo   `json:"page_info"`
	Total    *int       `json:"total,omitempty"` // Опционально: общее количество элементов
}

// NewConnection создаёт новое подключение
func NewConnection[T any](nodes []T, getCursor func(T) (*Cursor, error), pageRequest *PageRequest, hasMore bool) (*Connection[T], error) {
	edges := make([]*Edge[T], 0, len(nodes))

	for _, node := range nodes {
		cursor, err := getCursor(node)
		if err != nil {
			return nil, err
		}

		encodedCursor, err := cursor.Encode()
		if err != nil {
			return nil, err
		}

		edges = append(edges, &Edge[T]{
			Node:   node,
			Cursor: encodedCursor,
		})
	}

	pageInfo := PageInfo{
		HasNextPage:     hasMore && pageRequest.IsForward(),
		HasPreviousPage: hasMore && pageRequest.IsBackward(),
	}

	// Устанавливаем start и end курсоры
	if len(edges) > 0 {
		pageInfo.StartCursor = &edges[0].Cursor
		pageInfo.EndCursor = &edges[len(edges)-1].Cursor
	}

	return &Connection[T]{
		Edges:    edges,
		PageInfo: pageInfo,
	}, nil
}

// NewConnectionWithTotal создаёт подключение с общим количеством
func NewConnectionWithTotal[T any](nodes []T, getCursor func(T) (*Cursor, error), pageRequest *PageRequest, hasMore bool, total int) (*Connection[T], error) {
	conn, err := NewConnection(nodes, getCursor, pageRequest, hasMore)
	if err != nil {
		return nil, err
	}
	conn.Total = &total
	return conn, nil
}
