package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/netologist/go-discount-checkout/internal/audit"
	"github.com/netologist/go-discount-checkout/internal/checkout"
	"github.com/netologist/go-discount-checkout/internal/domain"
	"github.com/netologist/go-discount-checkout/internal/pipeline"
	"github.com/netologist/go-discount-checkout/internal/promotion"
	"github.com/netologist/go-discount-checkout/pkg/money"
)

func main() {
	// 1. Structured logger setup (Go 1.21+ log/slog).
	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	slogLogger := slog.New(jsonHandler)
	auditLogger := audit.NewSlogLogger(slogLogger)

	fmt.Println("==========================================================")
	fmt.Println("   Discount & Checkout Engine (Modern Go Native)")
	fmt.Println("==========================================================")

	// 2. Configure the Checkout engine with Functional Options & Composition.
	engine := checkout.New(
		checkout.WithLogger(auditLogger),
		checkout.WithPromotions(
			// Promotion 1: Buy 2 Get 1 Free on toothpaste.
			promotion.BOGOF("BOGOF Toothpaste", "PASTE"),

			// Promotion 2: 10% off orders of £50 or more.
			promotion.PercentOffOver("10% over £50", 10, money.GBP("50.00")),

			// Promotion 3: 5% off the whole basket for loyalty-card members.
			promotion.MemberPercentOff("Clubcard 5%", 5),

			// Promotion 4: £20 off exclusively for Gold-tier VIP members.
			promotion.GoldFixedOff("Gold £20 Off", money.GBP("20.00")),

			// Promotion 5: £5 off with coupon code SAVE5 (minimum spend £30).
			promotion.CouponFixedOff("Voucher SAVE5 (£5 off)", "SAVE5", money.GBP("5.00"), money.GBP("30.00")),
		),
	)

	// 3. Build a sample order (Gold member + coupon) using the thread-safe domain model.
	shopper := domain.Member("cust-gold-007", domain.TierGold)
	order := domain.GBPOrder(shopper).
		MustAddLine("PASTE", "Colgate Toothpaste", money.GBP("1.50"), 6). // 6 × £1.50 = £9.00 (BOGOF: 2 free = £3.00)
		MustAddLine("STEAK", "Prime Ribeye Steak", money.GBP("12.00"), 5). // 5 × £12.00 = £60.00
		MustAddLine("WINE", "Chateau Margaux", money.GBP("35.00"), 1).    // 1 × £35.00 = £35.00  (Subtotal = £104.00)
		MustRedeemCoupon("SAVE5")

	fmt.Printf("\n[Basket Summary] Customer: %s | Lines: %d | Gross Total: %s\n",
		order.Customer().ID, order.ItemCount(), order.Subtotal())

	// 4. Calculate the checkout total (context-aware & thread-safe).
	ctx := context.Background()
	receipt, err := engine.Total(ctx, order)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// 5. Print the receipt (Go 1.23+ iterators).
	fmt.Println("\n--------------------- SHOPPING RECEIPT ----------------------")
	for line := range receipt.LinesSeq() {
		fmt.Printf(" - %-22s %2d x %s = %s\n",
			line.Name, line.Quantity, line.UnitPrice, line.LineTotal())
	}
	fmt.Println("--------------------------------------------------------------")
	fmt.Printf(" Subtotal:                   %s\n", receipt.Subtotal)
	fmt.Println(" Applied Discounts:")
	for d := range receipt.DiscountsSeq() {
		fmt.Printf("   * %-28s -%s\n", d.PromotionName, d.Saving)
	}
	fmt.Printf(" Total Savings:              -%s\n", receipt.TotalSavings)
	fmt.Printf(" Amount Payable:              %s\n", receipt.Payable)
	fmt.Printf(" Summary: %s\n", receipt.Summary())
	fmt.Println("==========================================================")

	// 6. Concurrent Worker Pool pipeline demo.
	fmt.Println("\n[Concurrent Pipeline Demo] Processing 20 orders across 4 goroutine workers...")
	batchProcessor := pipeline.NewBatchProcessor(engine, 4)

	batchOrders := make([]*domain.Order, 20)
	for i := range 20 {
		c := domain.Guest(fmt.Sprintf("guest-%02d", i+1))
		batchOrders[i] = domain.GBPOrder(c).
			MustAddLine("STEAK", "Steak", money.GBP("12.00"), (i%4)+1)
	}

	results, err := batchProcessor.ProcessBatch(ctx, batchOrders)
	if err != nil {
		fmt.Printf("Batch error: %v\n", err)
		return
	}
	fmt.Printf("✅ Done: %d baskets processed concurrently. Total processed orders: %d\n",
		len(results), batchProcessor.ProcessedCount())
}
