package repository

import (
	"context"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/userattributedefinition"
	"github.com/Wei-Shaw/sub2api/ent/userattributevalue"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type cardsIssueBindingRepository struct {
	client *dbent.Client
}

func NewCardsIssueBindingRepository(client *dbent.Client) service.CardsIssueBindingRepository {
	return &cardsIssueBindingRepository{client: client}
}

func (r *cardsIssueBindingRepository) FindUserIDByBuyerID(ctx context.Context, buyerID string) (int64, bool, error) {
	definitionID, ok, err := r.lookupDefinitionID(ctx)
	if err != nil || !ok {
		return 0, false, err
	}
	values, err := clientFromContext(ctx, r.client).UserAttributeValue.Query().
		Where(
			userattributevalue.AttributeIDEQ(definitionID),
			userattributevalue.ValueEQ(buyerID),
		).
		Order(dbent.Asc(userattributevalue.FieldUserID)).
		All(ctx)
	if err != nil {
		return 0, false, err
	}
	if len(values) == 0 {
		return 0, false, nil
	}
	return values[0].UserID, true, nil
}

func (r *cardsIssueBindingRepository) BindBuyerID(ctx context.Context, userID int64, buyerID string) error {
	definitionID, err := r.ensureDefinitionID(ctx)
	if err != nil {
		return err
	}
	return clientFromContext(ctx, r.client).UserAttributeValue.Create().
		SetUserID(userID).
		SetAttributeID(definitionID).
		SetValue(buyerID).
		OnConflictColumns(userattributevalue.FieldUserID, userattributevalue.FieldAttributeID).
		UpdateValue().
		UpdateUpdatedAt().
		Exec(ctx)
}

func (r *cardsIssueBindingRepository) lookupDefinitionID(ctx context.Context) (int64, bool, error) {
	definition, err := clientFromContext(ctx, r.client).UserAttributeDefinition.Query().
		Where(userattributedefinition.KeyEQ(service.CardsIssueBuyerIDAttributeKey)).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return definition.ID, true, nil
}

func (r *cardsIssueBindingRepository) ensureDefinitionID(ctx context.Context) (int64, error) {
	if id, ok, err := r.lookupDefinitionID(ctx); err != nil {
		return 0, err
	} else if ok {
		return id, nil
	}

	definition, err := clientFromContext(ctx, r.client).UserAttributeDefinition.Create().
		SetKey(service.CardsIssueBuyerIDAttributeKey).
		SetName(service.CardsIssueBuyerIDAttributeName).
		SetDescription("Reserved system attribute for cards issue buyer mapping").
		SetType(string(service.AttributeTypeText)).
		SetPlaceholder("").
		SetRequired(false).
		SetEnabled(false).
		Save(ctx)
	if err != nil {
		if !dbent.IsConstraintError(err) {
			return 0, err
		}
		definition, err = clientFromContext(ctx, r.client).UserAttributeDefinition.Query().
			Where(userattributedefinition.KeyEQ(service.CardsIssueBuyerIDAttributeKey)).
			Only(ctx)
		if err != nil {
			return 0, err
		}
	}
	return definition.ID, nil
}
