# immutable-nodes-and-pointer-keyed-storage

Replace lock-guarded map[any]any nodes with immutable, pointer-keyed slice nodes; drop the read-path lock and re-measure
