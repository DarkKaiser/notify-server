package navershopping

import "fmt"

type ProductBuilder struct {
	product product
}

func NewProductBuilder() *ProductBuilder {
	return &ProductBuilder{
		product: product{
			Title:     "Default Title",
			Link:      "http://default.com",
			LowPrice:  1000,
			MallName:  "Naver",
			ProductID: "12345",
		},
	}
}

func (b *ProductBuilder) WithID(id string) *ProductBuilder {
	b.product.ProductID = id
	return b
}
func (b *ProductBuilder) WithTitle(t string) *ProductBuilder {
	b.product.Title = t
	return b
}
func (b *ProductBuilder) WithPrice(p int) *ProductBuilder {
	b.product.LowPrice = p
	return b
}
func (b *ProductBuilder) WithLink(l string) *ProductBuilder {
	b.product.Link = l
	return b
}
func (b *ProductBuilder) WithMallName(m string) *ProductBuilder {
	b.product.MallName = m
	return b
}
func (b *ProductBuilder) Build() *product {
	return &b.product
}

// makeMockProducts 테스트용 상품 목록을 대량으로 생성하는 헬퍼 함수
func makeMockProducts(count int) []*product {
	products := make([]*product, count)
	for i := 0; i < count; i++ {
		id := fmt.Sprintf("%d", i)
		products[i] = NewProductBuilder().
			WithID(id).
			WithTitle(fmt.Sprintf("Product %d", i)).
			WithPrice(1000 + i).
			Build()
	}
	return products
}
