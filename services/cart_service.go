package services

import (
	"errors"

	"bubblewhite-backend/models"
	"bubblewhite-backend/repositories"
)

type CartService struct {
	Cart     *repositories.CartRepository
	Products *repositories.ProductRepository
}

func NewCartService(cart *repositories.CartRepository, products *repositories.ProductRepository) *CartService {
	return &CartService{Cart: cart, Products: products}
}

// CartLineView is what the API actually returns for each cart line — the
// raw CartItem plus the product's current name/price/image, so the
// frontend doesn't need a second round-trip per line to render the cart.
type CartLineView struct {
	ID        uint    `json:"id"`
	ProductID string  `json:"productId"`
	Name      string  `json:"name"`
	Price     float64 `json:"price"`
	Image     string  `json:"image"`
	Size      string  `json:"size"`
	Quantity  int     `json:"quantity"`
}

var ErrProductNotFound = errors.New("product not found")
var ErrEmptyCart = errors.New("cart is empty")

// List returns the customer's cart, enriched with each product's current
// name/price — deliberately "current," not what was true when the item was
// added, since price/name can change and the cart should always reflect
// real inventory, not a stale snapshot. (Checkout, by contrast, DOES
// snapshot — see OrderService.Checkout — since a past order must never
// silently change.) Image is the ONE thing NOT taken from the live
// product: it uses whichever specific image the customer was previewing
// when they added it, falling back to the product's current first image
// only if none was captured.
func (s *CartService) List(customerID uint) ([]CartLineView, error) {
	lines, err := s.Cart.FindByCustomer(customerID)
	if err != nil {
		return nil, err
	}

	views := make([]CartLineView, 0, len(lines))
	for _, line := range lines {
		product, err := s.Products.FindByID(line.ProductID)
		if err != nil {
			// The product was deleted after being added to someone's cart —
			// skip it rather than fail the whole cart listing.
			continue
		}
		image := line.Image
		if image == "" && len(product.Images.Data) > 0 {
			image = product.Images.Data[0]
		}
		views = append(views, CartLineView{
			ID:        line.ID,
			ProductID: line.ProductID,
			Name:      product.Name,
			Price:     product.Price,
			Image:     image,
			Size:      line.Size,
			Quantity:  line.Quantity,
		})
	}
	return views, nil
}

// AddItem increments the existing line for this exact (product, size,
// image) combination if one exists, otherwise creates a new one — a
// different preview image is treated as a distinct line (see CartItem's
// doc comment), so ordering the same product in two different colors
// doesn't merge their quantities together.
func (s *CartService) AddItem(customerID uint, productID, size, image string, quantity int) error {
	if _, err := s.Products.FindByID(productID); err != nil {
		return ErrProductNotFound
	}

	existing, err := s.Cart.FindLine(customerID, productID, size, image)
	if err == nil {
		existing.Quantity += quantity
		return s.Cart.Update(existing)
	}

	item := &models.CartItem{CustomerID: customerID, ProductID: productID, Size: size, Image: image, Quantity: quantity}
	return s.Cart.Create(item)
}

// UpdateQuantity sets a line's quantity, deleting it outright if the result
// is zero or less (matches "press − until it's gone" UX).
func (s *CartService) UpdateQuantity(customerID, itemID uint, quantity int) error {
	item, err := s.Cart.FindByIDForCustomer(itemID, customerID)
	if err != nil {
		return err
	}
	if quantity <= 0 {
		return s.Cart.Delete(item.ID)
	}
	item.Quantity = quantity
	return s.Cart.Update(item)
}

func (s *CartService) RemoveItem(customerID, itemID uint) error {
	item, err := s.Cart.FindByIDForCustomer(itemID, customerID)
	if err != nil {
		return err
	}
	return s.Cart.Delete(item.ID)
}

func (s *CartService) Clear(customerID uint) error {
	return s.Cart.DeleteAllForCustomer(customerID)
}
