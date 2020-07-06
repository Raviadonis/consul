package state

import (
	"github.com/hashicorp/consul/agent/consul/state/db"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	memdb "github.com/hashicorp/go-memdb"
)

// aclChangeUnsubscribeEvent creates and returns stream.UnsubscribeEvents that
// are used to unsubscribe any subscriptions which match the tokens from the events.
//
// These are special events that will never be returned to a subscriber.
func aclChangeUnsubscribeEvent(tx db.ReadTxn, changes db.Changes) ([]stream.Event, error) {
	var secretIDs []string

	for _, change := range changes.Changes {
		switch change.Table {
		case "acl-tokens":
			token := changeObject(change).(*structs.ACLToken)
			secretIDs = append(secretIDs, token.SecretID)

		case "acl-roles":
			role := changeObject(change).(*structs.ACLRole)
			tokens, err := aclTokenListByRole(tx, role.ID, &role.EnterpriseMeta)
			if err != nil {
				return nil, err
			}
			secretIDs = appendSecretIDsFromTokenIterator(secretIDs, tokens)

		case "acl-policies":
			policy := changeObject(change).(*structs.ACLPolicy)
			tokens, err := aclTokenListByPolicy(tx, policy.ID, &policy.EnterpriseMeta)
			if err != nil {
				return nil, err
			}
			secretIDs = appendSecretIDsFromTokenIterator(secretIDs, tokens)

			roles, err := aclRoleListByPolicy(tx, policy.ID, &policy.EnterpriseMeta)
			if err != nil {
				return nil, err
			}
			for role := roles.Next(); role != nil; role = roles.Next() {
				role := role.(*structs.ACLRole)

				tokens, err := aclTokenListByRole(tx, role.ID, &policy.EnterpriseMeta)
				if err != nil {
					return nil, err
				}
				secretIDs = appendSecretIDsFromTokenIterator(secretIDs, tokens)
			}
		}
	}
	// TODO: should we remove duplicate IDs here, or rely on sub.Close() being idempotent
	return []stream.Event{stream.NewCloseSubscriptionEvent(secretIDs)}, nil
}

// changeObject returns the object before it was deleted if the change was a delete,
// otherwise returns the object after the change.
func changeObject(change memdb.Change) interface{} {
	if change.Deleted() {
		return change.Before
	}
	return change.After
}

func appendSecretIDsFromTokenIterator(seq []string, tokens memdb.ResultIterator) []string {
	for token := tokens.Next(); token != nil; token = tokens.Next() {
		token := token.(*structs.ACLToken)
		seq = append(seq, token.SecretID)
	}
	return seq
}