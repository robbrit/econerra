package agents

import (
	"math"

	"github.com/robbrit/econerra/goods"
	"github.com/robbrit/econerra/market"
)

// A Firm is an agent responsible for buying labour and producing goods.
type Firm struct {
	// What good this firm produces.
	goodProduced goods.Good
	// What wage this firm is hiring at.
	wage market.Price
	// What price this firm is selling its good at.
	price market.Price
	// How many workers were hired last iteration.
	workersHired market.Size
	// How many workers this firm wanted to hire last iteration.
	targetWorkers market.Size
	// How many sales were made last iteration.
	salesMade market.Size
	// How many sales this firm wanted to make last iteration.
	targetSales market.Size
}

// NewFirm creates a new firm with the given production parameters.
func NewFirm(goodProduced goods.Good, initialWage, initialPrice market.Price) *Firm {
	return &Firm{goodProduced, initialWage, initialPrice, 0, 0, 0, 0}
}

// TargetWorkers gets the number of workers that this firm is trying to hire
// this period.
func (f *Firm) TargetWorkers() market.Size { return f.targetWorkers }

// Act triggers the firm's decision process.
func (f *Firm) Act(p *Parameters) {
	// Step one: adjust our prices based on the results of last iteration.
	f.adjustPrices(p)
	// Step two: calculate profit maximizing values of quantity and labour based on calculated prices.
	f.chooseTargets(p)
	// Step three: send out orders to the markets.
	f.placeOrders(p)
	// Step four: before we start receiving fills for orders, clear the information we saved from
	// last iteration.
	f.reset()
}

func (f *Firm) adjustPrices(p *Parameters) {
	if f.targetSales == 0 {
		// This will happen in the first iteration, no need to adjust from the initial prices.
		return
	}

	goodInfo := p.Goods[f.goodProduced]

	// First adjust the price.
	if f.salesMade < f.targetSales {
		// Didn't sell enough, reduce price.
		f.price -= p.Increment
		// TODO(rob): A more intelligent agent would probably look at the market to make a
		// decision.
	} else if f.price <= goodInfo.Market.High() {
		// Made enough sales, bump up price to the maximum of what was selling last round.
		f.price = goodInfo.Market.High() + p.Increment
	}

	// Now adjust the wage.
	if f.workersHired < f.targetWorkers {
		// Didn't hire enough people, offer a better wage than the market.
		f.wage = p.LabourMarket.High() + p.Increment
	} else if f.wage >= p.LabourMarket.Low() {
		// Got enough people, lower wages if possible
		// TODO(rob): A more intelligent agent would probably look at the market to make a
		// decision.
		f.wage -= p.Increment
	}
}

func (f *Firm) chooseTargets(p *Parameters) {
	goodInfo := p.Goods[f.goodProduced]

	/*
		Using a Cobb-Douglas production function:

		   Q = Tech * L^Scale

		the profit maximizing labour is:

		   L = (wage / (price * scale * tech))^(1 / (scale - 1))

	*/
	base := float64(f.wage) / (float64(f.price) * goodInfo.Tech * goodInfo.Scale)
	exp := 1.0 / (goodInfo.Scale - 1.0)
	targetLabour := math.Pow(base, exp)

	// Since labour is discrete, need to see which of the ceiling or floor gives better profits.
	if f.profits(p, math.Ceil(targetLabour)) > f.profits(p, math.Floor(targetLabour)) {
		f.targetWorkers = market.Size(math.Ceil(targetLabour))
	} else {
		f.targetWorkers = market.Size(math.Floor(targetLabour))
	}

	if f.workersHired > 0 {
		// Can only produce if we managed to hire workers last iteration.
		// Note that this will produce a lag between prices and wages.
		f.targetSales = market.Size(math.Floor(f.production(p, float64(f.workersHired))))
	}
}

func (f *Firm) placeOrders(p *Parameters) {
	if f.targetWorkers > 0 {
		p.LabourMarket.Post(&market.Order{
			Price: f.wage,
			Size:  f.targetWorkers,
			Side:  market.Buy,
			Owner: f,
		})
	}

	if f.targetSales > 0 {
		goodInfo := p.Goods[f.goodProduced]
		goodInfo.Market.Post(&market.Order{
			Price: f.price,
			Size:  f.targetSales,
			Side:  market.Sell,
			Owner: f,
		})
	}
}

func (f *Firm) reset() {
	f.workersHired = 0
	f.salesMade = 0
}

// profits calculates how much profit a firm makes given a wage and target labour.
// Note that this is expected profits - it's possible the firm will not sell all the goods it
// produces.
func (f *Firm) profits(p *Parameters, labour float64) float64 {
	wage := float64(f.wage)
	price := float64(f.price)
	return price*f.production(p, labour) - float64(wage)*labour
}

// production calculates how much the firm produces with a given amount of labour.
func (f *Firm) production(p *Parameters, labour float64) float64 {
	goodInfo := p.Goods[f.goodProduced]
	return goodInfo.Tech * math.Pow(labour, goodInfo.Scale)
}

// OnFill is triggered when the firm makes a sale.
func (f *Firm) OnFill(good goods.Good, side market.Side, wage market.Price, size market.Size) {
	if good == goods.Labour {
		f.workersHired += size
	} else if good == f.goodProduced {
		f.salesMade += size
	}
}

// OnUnfilled is triggered if the firm has unfilled orders at the end of the iteration.
func (f *Firm) OnUnfilled(goods.Good, market.Side, market.Size) {
	// No need to actually do anything here.
}
