package wossamessa

type pulseDetector struct {
	lastRaw          int
	lastHighLow      int
	lastPulseHighLow int
}

func (p *pulseDetector) process(sum int) bool {
	highLow := p.readHighLow(sum)
	pulse := p.readPulse(highLow)

	return pulse > 0
}

func (p *pulseDetector) readHighLow(sum int) int {
	config, _ := loadConfig()
	triggerHigh := config.TriggerHigh
	triggerLow := config.TriggerLow

	result := 0
	if p.lastRaw <= triggerHigh && sum > triggerHigh {
		result = 1
	} else if p.lastRaw >= triggerLow && sum < triggerLow {
		result = 0
	} else {
		result = p.lastHighLow
	}
	p.lastHighLow = result
	p.lastRaw = sum

	return result
}

func (p *pulseDetector) readPulse(highLow int) int {
	//
	//  1 - 0 =  1 --> RISING
	//  1 - 1 =  0
	//  0 - 1 = -1 --> FALLING
	//  0 - 0 =  0
	//
	flank := highLow - p.lastPulseHighLow
	p.lastPulseHighLow = highLow

	return flank
}
