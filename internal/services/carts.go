package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/mohammadne/shopping-cart-manager/internal/entities"
	"github.com/mohammadne/shopping-cart-manager/internal/repositories/storage"
)

type Carts interface {
	RetrieveCartOptional(ctx context.Context, cookie string) (*entities.Cart, error)
	RetrieveCartRequired(ctx context.Context, cookie string) (*entities.Cart, error)
	AllCartItemsByCartId(ctx context.Context, cartId int) ([]entities.CartItem, error)
	AddItemToCart(ctx context.Context, cartId, itemId, quantity int) error
	DeleteCartItem(ctx context.Context, cartId, cartItemId int) (err error)
}

func NewCarts(cartItemsStorage storage.CartItems, cartsStorage storage.Carts, itemsStorage storage.Items) Carts {
	return &carts{cartItemsStorage: cartItemsStorage, cartsStorage: cartsStorage, itemsStorage: itemsStorage}
}

type carts struct {
	cartItemsStorage storage.CartItems
	cartsStorage     storage.Carts
	itemsStorage     storage.Items
}

func (u *carts) RetrieveCartOptional(ctx context.Context, cookie string) (result *entities.Cart, err error) {
	{ // validation
		if cookie == "" {
			return nil, errors.New("the cookie should be provided")
		}
	}

	storageUser, err := u.cartsStorage.RetrieveCartByCookieAndStatus(ctx, cookie, entities.CartStatusOpen)
	if err != nil {
		if err == storage.ErrorCartNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error retrieving cart: %v", err)
	}

	return &entities.Cart{
		Id:     storageUser.ID,
		Cookie: storageUser.Cookie,
	}, nil
}

func (u *carts) RetrieveCartRequired(ctx context.Context, cookie string) (result *entities.Cart, err error) {
	{ // validation
		if cookie == "" {
			return nil, errors.New("the cookie should be provided")
		}
	}

	storageCart, err := u.cartsStorage.RetrieveCartByCookieAndStatus(ctx, cookie, entities.CartStatusOpen)
	if err != nil {
		if err == storage.ErrorCartNotFound {
			storageCart := &storage.Cart{
				Cookie:    cookie,
				Status:    string(entities.CartStatusOpen),
				CreatedAt: time.Now(),
			}

			id, err := u.cartsStorage.CreateCart(ctx, storageCart)
			if err != nil {
				return nil, fmt.Errorf("error creating cart: %v", err)
			}
			return &entities.Cart{
				Id:     id,
				Cookie: cookie,
				Status: entities.CartStatusOpen,
			}, nil
		}
		return nil, fmt.Errorf("error retrieving cart: %v", err)
	}

	return &entities.Cart{
		Id:     storageCart.ID,
		Cookie: storageCart.Cookie,
		Status: entities.CartStatus(storageCart.Status),
	}, nil
}

func (c *carts) AllCartItemsByCartId(ctx context.Context, cartId int) ([]entities.CartItem, error) {
	storageCartItems, err := c.cartItemsStorage.AllCartItemsByCartId(ctx, cartId)
	if err != nil {
		if err == storage.ErrorCartItemNotFound {
			return []entities.CartItem{}, nil
		}
		return nil, fmt.Errorf("error retrieving cart-items by cart-id: %v", err)
	} else if len(storageCartItems) == 0 {
		return []entities.CartItem{}, nil
	}

	itemIds := make([]int, 0, len(storageCartItems))
	for _, storageCartItem := range storageCartItems {
		itemIds = append(itemIds, storageCartItem.ItemID)
	}

	storageItems, err := c.itemsStorage.AllItemsByItemIds(ctx, itemIds)
	if err != nil {
		if err == storage.ErrorCartItemNotFound {
			return []entities.CartItem{}, nil
		}
		return nil, fmt.Errorf("error retrieving items by item-ids: %v", err)
	}

	cartItems := make([]entities.CartItem, 0, len(storageCartItems))
	for _, storageCartItem := range storageCartItems {
		if storageCartItem.DeletedAt.Valid {
			continue
		} else if storageCartItem.Quantity <= 0 {
			continue
		}

		cartItem := entities.CartItem{
			Id:        storageCartItem.ID,
			Cart:      &entities.Cart{},
			Quantity:  storageCartItem.Quantity,
			IsDeleted: storageCartItem.DeletedAt.Valid,
		}

		var item *entities.Item
		for _, storageItem := range storageItems {
			if storageCartItem.ItemID == storageItem.ID {
				item = &entities.Item{
					Id:    storageItem.ID,
					Name:  storageItem.Name,
					Price: storageItem.Price,
				}
			}
		}
		cartItem.Item = item

		cartItems = append(cartItems, cartItem)
	}

	return cartItems, nil
}

func (c *carts) AddItemToCart(ctx context.Context, cartId, itemId, quantity int) (err error) {
	storageCartItem, err := c.cartItemsStorage.RetrieveCartItemByCartIdAndItemId(ctx, cartId, itemId)
	if err != nil {
		if err == storage.ErrorCartItemNotFound {
			storageCartItem := storage.CartItem{
				CartID:    cartId,
				ItemID:    itemId,
				Quantity:  quantity,
				CreatedAt: time.Now(),
			}

			_, err = c.cartItemsStorage.CreateCartItem(ctx, &storageCartItem)
			if err != nil {
				return fmt.Errorf("error creating cart-item: %v", err)
			}

			return
		}
		return fmt.Errorf("error retrieving cart-item by cart-id and item-id: %v", err)
	}

	if storageCartItem.DeletedAt.Valid {
		storageCartItem := storage.CartItem{
			CartID:    cartId,
			ItemID:    itemId,
			Quantity:  quantity,
			CreatedAt: time.Now(),
		}

		_, err = c.cartItemsStorage.CreateCartItem(ctx, &storageCartItem)
		if err != nil {
			return fmt.Errorf("error creating cart-item: %v", err)
		}
	} else {
		storageCartItem.Quantity += quantity
		storageCartItem.UpdatedAt = sql.NullTime{Time: time.Now(), Valid: true}
		err = c.cartItemsStorage.UpdateCartItem(ctx, storageCartItem)
		if err != nil {
			return fmt.Errorf("error updating cart-item quantity: %v", err)
		}
	}

	return nil
}

var ErrorCartHasBeenClosed = errors.New("the cart has been closed")

func (c *carts) DeleteCartItem(ctx context.Context, cartId, cartItemId int) (err error) {
	storageCart, err := c.cartsStorage.RetrieveCartById(ctx, cartId)
	if err != nil {
		return fmt.Errorf("error retrieving cart: %v", err)
	}

	if storageCart.Status == string(entities.CartStatusClosed) {
		return ErrorCartHasBeenClosed
	}

	err = c.cartItemsStorage.DeleteCartItemById(ctx, cartItemId, time.Now())
	if err != nil {
		return fmt.Errorf("error deleting cart-item: %v", err)
	}

	return nil
}
