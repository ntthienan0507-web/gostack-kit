package database

import "gorm.io/gorm/clause"

// ForUpdate adds SELECT ... FOR UPDATE to a GORM query.
// Use inside a transaction to lock rows until commit/rollback.
//
// When to use:
//   - Deducting balance, stock, quota (read -> compute -> write must be atomic)
//   - Any read-modify-write where the value read MUST NOT change before write
//   - Conflict is FREQUENT (optimistic locking would cause too many retries)
//
// When NOT to use:
//   - Updating descriptive fields (name, note) -> use optimistic locking
//   - Read-only queries -> no lock needed
//   - Long-running operations -> lock blocks other transactions
//
// Example — deduct inventory:
//
//	database.WithTransaction(ctx, db, logger, func(tx *gorm.DB) error {
//	    var product Product
//	    if err := tx.Clauses(ForUpdate()).Where("id = ?", id).First(&product).Error; err != nil {
//	        return err
//	    }
//	    if product.Stock < quantity {
//	        return ErrInsufficientStock
//	    }
//	    return tx.Model(&product).Update("stock", product.Stock - quantity).Error
//	})
func ForUpdate() clause.Locking {
	return clause.Locking{Strength: "UPDATE"}
}

// ForUpdateNoWait adds FOR UPDATE NOWAIT — fails immediately if the row is already locked
// by another transaction, instead of waiting for the lock to be released.
//
// When to use:
//   - Payment processing: fail fast and return an error rather than queue up behind
//     another transaction holding the lock. The client can retry or show an error.
//   - High-throughput endpoints where blocking would cascade into request timeouts.
//   - Any scenario where you'd rather reject the request than risk holding an HTTP
//     connection open while waiting for a row lock.
//
// Example — charge a wallet (fail fast):
//
//	database.WithTransaction(ctx, db, logger, func(tx *gorm.DB) error {
//	    var wallet Wallet
//	    err := tx.Clauses(ForUpdateNoWait()).Where("user_id = ?", userID).First(&wallet).Error
//	    if err != nil {
//	        // If another transaction holds the lock, Postgres returns:
//	        //   ERROR: could not obtain lock on row in relation "wallets"
//	        return err
//	    }
//	    if wallet.Balance < amount {
//	        return ErrInsufficientBalance
//	    }
//	    return tx.Model(&wallet).Update("balance", wallet.Balance - amount).Error
//	})
func ForUpdateNoWait() clause.Locking {
	return clause.Locking{Strength: "UPDATE", Options: "NOWAIT"}
}

// ForUpdateSkipLocked adds FOR UPDATE SKIP LOCKED — silently skips rows that are
// already locked by another transaction instead of waiting or failing.
//
// When to use:
//   - Job queue / task table: multiple workers SELECT the next N unlocked rows,
//     each worker gets a different batch with zero contention.
//   - Outbox relay: pick unpublished events without blocking other relay instances.
//   - Any "claim next available item" pattern where it's fine to skip busy rows.
//
// Example — claim next pending job:
//
//	database.WithTransactionResult[*Job](ctx, db, logger, func(tx *gorm.DB) (*Job, error) {
//	    var job Job
//	    err := tx.Clauses(ForUpdateSkipLocked()).
//	        Where("status = ?", "pending").
//	        Order("created_at ASC").
//	        First(&job).Error
//	    if err != nil {
//	        return nil, err // no unlocked pending jobs
//	    }
//	    job.Status = "processing"
//	    if err := tx.Save(&job).Error; err != nil {
//	        return nil, err
//	    }
//	    return &job, nil
//	})
func ForUpdateSkipLocked() clause.Locking {
	return clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}
}

// ForShare adds FOR SHARE — acquires a shared lock that allows concurrent readers
// but blocks writers until the transaction completes.
//
// When to use:
//   - Report queries that must see a consistent snapshot across multiple rows
//     (e.g., summing balances across accounts — no row should change mid-query).
//   - Validating referential data: lock the parent row with FOR SHARE so it can't
//     be deleted while you insert a child row, without blocking other readers.
//   - Any read where you need a guarantee the data won't be modified, but you don't
//     intend to modify it yourself.
//
// Example — consistent balance report:
//
//	database.WithTransactionResult[[]Account](ctx, db, logger, func(tx *gorm.DB) ([]Account, error) {
//	    var accounts []Account
//	    err := tx.Clauses(ForShare()).Where("org_id = ?", orgID).Find(&accounts).Error
//	    if err != nil {
//	        return nil, err
//	    }
//	    // All accounts are share-locked: no concurrent UPDATE/DELETE can proceed
//	    // until this transaction commits, ensuring a consistent sum.
//	    return accounts, nil
//	})
func ForShare() clause.Locking {
	return clause.Locking{Strength: "SHARE"}
}
