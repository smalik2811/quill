package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/smalik2811/quill/internal/quill"
)

func main() {
	htmlPath := flag.String("html", "curve.html", "Path to write HTML visualization (leave empty to skip)")
	maxDuration := flag.Duration("duration", 1000*time.Millisecond, "Maximum hold duration to graph")
	flag.Parse()

	cfg := quill.GetCurveConfig()

	fmt.Println("================================================================================")
	fmt.Println("                      QUILL MOVEMENT CURVE CONFIGURATION                        ")
	fmt.Println("================================================================================")
	fmt.Printf("  Start Speed (v0):        %8.1f units/s\n", cfg.StartSpeed)
	fmt.Printf("  Max Speed (vmax):        %8.1f units/s\n", cfg.MaxSpeed)
	fmt.Printf("  Timescale (tau):         %8v\n", cfg.TimeConstant)
	fmt.Printf("  Curve Power (p):         %8.1f (p = 2.0 = Quadratic acceleration)\n", cfg.Power)
	fmt.Printf("  Precise Mode Speed:      %8.1f units/s\n", cfg.PreciseSpeed)
	fmt.Printf("  Initial Press Step:      %8d units\n", cfg.InitialPressStep)
	fmt.Printf("  Max Step Interval (dt):  %8v\n", cfg.MaxStepInterval)
	fmt.Println("================================================================================")
	fmt.Println()

	// 1. ASCII Plot of Speed vs Time
	printAsciiPlot(*maxDuration, cfg)

	// 2. Sample Data Table with Effective Speed
	printDataTable(*maxDuration)

	// 3. HTML File Generation
	if *htmlPath != "" {
		if err := generateHTMLChart(*htmlPath, *maxDuration, cfg); err != nil {
			fmt.Printf("Error generating HTML chart: %v\n", err)
		} else {
			fmt.Printf("Interactive visualization saved to: %s\n", *htmlPath)
			fmt.Printf("Open it with:  xdg-open %s\n\n", *htmlPath)
		}
	}
}

func printAsciiPlot(maxDur time.Duration, cfg quill.CurveConfig) {
	fmt.Println("--- SPEED VS HOLD DURATION (Normal Mode Quadratic Acceleration) ---")
	fmt.Println("  Time (ms) | Speed (units/s)                       0                          2000 units/s")
	fmt.Println(" -----------+------------------------------------+-----------------------------------------")

	const rows = 20
	const barWidth = 40
	step := maxDur / rows

	for i := 0; i <= rows; i++ {
		t := time.Duration(i) * step
		spd := quill.EvaluateNormalCurve(t)
		barLen := int(math.Round((spd / cfg.MaxSpeed) * barWidth))
		if barLen < 0 {
			barLen = 0
		}
		if barLen > barWidth {
			barLen = barWidth
		}
		bar := strings.Repeat("█", barLen) + strings.Repeat("░", barWidth-barLen)
		fmt.Printf("  %6d ms  | %7.1f u/s  |%s|\n", t.Milliseconds(), spd, bar)
	}
	fmt.Println(" -----------+------------------------------------+-----------------------------------------")
	fmt.Println()
}

func computeDistanceAt(t time.Duration) float64 {
	if t <= 0 {
		return 0
	}
	const slices = 200
	dt := t / slices
	dtSec := dt.Seconds()
	var dist float64
	for i := 0; i < slices; i++ {
		tMid := (time.Duration(i)*dt + time.Duration(i+1)*dt) / 2
		dist += quill.EvaluateNormalCurve(tMid) * dtSec
	}
	return dist
}

func printDataTable(maxDur time.Duration) {
	fmt.Println("--- DETAILED SAMPLE POINTS (Including Effective Traversal Speed) ---")
	fmt.Println("  Hold Time | Normal Speed | Effective Avg | Precise Speed | Total Travel | Emitted Step (dt=50ms)")
	fmt.Println(" -----------+--------------+---------------+---------------+--------------+------------------------")

	checkpoints := []time.Duration{
		0,
		25 * time.Millisecond,
		50 * time.Millisecond,
		75 * time.Millisecond,
		100 * time.Millisecond,
		150 * time.Millisecond,
		200 * time.Millisecond,
		250 * time.Millisecond,
		300 * time.Millisecond,
		400 * time.Millisecond,
		500 * time.Millisecond,
		750 * time.Millisecond,
		1000 * time.Millisecond,
	}

	for _, t := range checkpoints {
		if t > maxDur {
			break
		}
		spd := quill.EvaluateNormalCurve(t)
		prec := quill.EvaluatePreciseCurve(t)
		dist := computeDistanceAt(t)
		var effSpeed float64
		if t > 0 {
			effSpeed = dist / t.Seconds()
		} else {
			effSpeed = spd
		}
		step50 := spd * 0.050 // REL units for a typical 50ms interval
		fmt.Printf("   %5d ms  | %8.1f u/s | %9.1f u/s | %9.1f u/s | %8.1f units |    %6.1f REL units\n",
			t.Milliseconds(), spd, effSpeed, prec, dist, step50)
	}
	fmt.Println(" -----------+--------------+---------------+---------------+--------------+------------------------")
	fmt.Println()
}

func generateHTMLChart(path string, maxDur time.Duration, cfg quill.CurveConfig) error {
	const samples = 100
	step := maxDur / samples

	var labels []string
	var normalData []string
	var effSpeedData []string
	var preciseData []string
	var accelData []string
	var edgeFrictionData []string

	totalDim := 1440.0 // standard logical width
	distToEdge := 50.0 // near edge (50px)

	var cumulativeDist float64
	var effSpeedAt500ms float64
	var totalDistAtMax float64

	for i := 0; i <= samples; i++ {
		t := time.Duration(i) * step
		ms := t.Milliseconds()
		spd := quill.EvaluateNormalCurve(t)
		prec := quill.EvaluatePreciseCurve(t)

		// Cumulative distance and effective average speed (v_eff = dist / t)
		if i > 0 {
			prevT := time.Duration(i-1) * step
			prevSpd := quill.EvaluateNormalCurve(prevT)
			dtSec := (t - prevT).Seconds()
			cumulativeDist += ((prevSpd + spd) / 2.0) * dtSec
		}

		var effSpeed float64
		if t > 0 {
			effSpeed = cumulativeDist / t.Seconds()
		} else {
			effSpeed = spd
		}

		if ms == 500 || (effSpeedAt500ms == 0 && ms >= 500) {
			effSpeedAt500ms = effSpeed
		}
		totalDistAtMax = cumulativeDist

		// Acceleration: dv/dt
		var accel float64
		if i > 0 {
			prevT := time.Duration(i-1) * step
			prevSpd := quill.EvaluateNormalCurve(prevT)
			dtSec := (t - prevT).Seconds()
			if dtSec > 0 {
				accel = (spd - prevSpd) / dtSec
			}
		}

		// Edge friction damped speed near screen edge (50px from edge)
		f := quill.CalculateFriction(distToEdge, totalDim, t)
		spdDamped := spd * (1.0 - f)

		labels = append(labels, fmt.Sprintf("%dms", ms))
		normalData = append(normalData, fmt.Sprintf("%.1f", spd))
		effSpeedData = append(effSpeedData, fmt.Sprintf("%.1f", effSpeed))
		preciseData = append(preciseData, fmt.Sprintf("%.1f", prec))
		accelData = append(accelData, fmt.Sprintf("%.1f", accel))
		edgeFrictionData = append(edgeFrictionData, fmt.Sprintf("%.1f", spdDamped))
	}

	htmlContent := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Quill Movement Curve Graph</title>
  <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
  <style>
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
      background-color: #0f141c;
      color: #e6edf3;
      margin: 0;
      padding: 24px;
    }
    .container {
      max-width: 1100px;
      margin: 0 auto;
    }
    h1 {
      margin-bottom: 4px;
      color: #58a6ff;
    }
    .subtitle {
      color: #8b949e;
      margin-bottom: 24px;
    }
    .card {
      background: #161b22;
      border: 1px solid #30363d;
      border-radius: 8px;
      padding: 20px;
      margin-bottom: 24px;
      box-shadow: 0 4px 12px rgba(0,0,0,0.3);
    }
    .stats-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
      gap: 16px;
      margin-bottom: 24px;
    }
    .stat-box {
      background: #21262d;
      border-radius: 6px;
      padding: 14px;
      border-left: 4px solid #58a6ff;
    }
    .stat-label {
      color: #8b949e;
      font-size: 12px;
      text-transform: uppercase;
      letter-spacing: 0.5px;
    }
    .stat-value {
      font-size: 20px;
      font-weight: 600;
      color: #f0f6fc;
      margin-top: 4px;
    }
    .chart-wrapper {
      position: relative;
      height: 480px;
      width: 100%%;
    }
    .legend-desc {
      color: #8b949e;
      font-size: 13px;
      margin-top: 14px;
      line-height: 1.6;
    }
  </style>
</head>
<body>
  <div class="container">
    <h1>Quill Movement Velocity & Acceleration Curve</h1>
    <div class="subtitle">Live visualization of instantaneous velocity, cumulative effective speed, and edge-friction physics</div>

    <div class="stats-grid">
      <div class="stat-box" style="border-left-color: #58a6ff;">
        <div class="stat-label">Start Speed (v0)</div>
        <div class="stat-value">%.1f <span style="font-size: 13px; color: #8b949e;">units/s</span></div>
      </div>
      <div class="stat-box" style="border-left-color: #3fb950;">
        <div class="stat-label">Max Speed (vmax)</div>
        <div class="stat-value">%.1f <span style="font-size: 13px; color: #8b949e;">units/s</span></div>
      </div>
      <div class="stat-box" style="border-left-color: #2ea043;">
        <div class="stat-label">Effective Avg (at 500ms)</div>
        <div class="stat-value">%.1f <span style="font-size: 13px; color: #8b949e;">units/s</span></div>
      </div>
      <div class="stat-box" style="border-left-color: #38bdf8;">
        <div class="stat-label">Total Distance (1s Hold)</div>
        <div class="stat-value">%.0f <span style="font-size: 13px; color: #8b949e;">units</span></div>
      </div>
      <div class="stat-box" style="border-left-color: #d29922;">
        <div class="stat-label">Ramp Timescale (&tau;)</div>
        <div class="stat-value">%v</div>
      </div>
      <div class="stat-box" style="border-left-color: #a371f7;">
        <div class="stat-label">Curve Power (p)</div>
        <div class="stat-value">%.1f <span style="font-size: 13px; color: #8b949e;">(Quadratic)</span></div>
      </div>
      <div class="stat-box" style="border-left-color: #f0883e;">
        <div class="stat-label">Initial Nudge</div>
        <div class="stat-value">%d <span style="font-size: 13px; color: #8b949e;">units</span></div>
      </div>
    </div>

    <div class="card">
      <div class="chart-wrapper">
        <canvas id="curveChart"></canvas>
      </div>
      <div class="legend-desc">
        <strong>Curves explanation:</strong><br>
        &bull; <span style="color: #58a6ff;"><strong>Normal Speed (Instantaneous):</strong></span> Instantaneous velocity approaching 2000 units/s.<br>
        &bull; <span style="color: #2ea043;"><strong>Effective Speed (Cumulative Average):</strong></span> True average speed across the hold (&int;v dt &divide; t), representing the actual pace at which you traverse the screen.<br>
        &bull; <span style="color: #f0883e;"><strong>Near Screen Edge (Damped):</strong></span> Shows dynamic friction braking when moving towards a border (at 50px distance).<br>
        &bull; <span style="color: #38bdf8;"><strong>Precise Mode:</strong></span> Constant 35 units/s for single-pixel precision tasks.<br>
        &bull; <span style="color: #a371f7;"><strong>Acceleration (dv/dt):</strong></span> Shows acceleration peaking during the mid-ramp and easing off to zero as top speed is approached.<br>
      </div>
    </div>
  </div>

  <script>
    const ctx = document.getElementById('curveChart').getContext('2d');
    new Chart(ctx, {
      type: 'line',
      data: {
        labels: [%s],
        datasets: [
          {
            label: 'Normal Speed (Instantaneous u/s)',
            data: [%s],
            borderColor: '#58a6ff',
            backgroundColor: 'rgba(88, 166, 255, 0.08)',
            borderWidth: 3,
            tension: 0.2,
            yAxisID: 'y',
            fill: true
          },
          {
            label: 'Effective Speed (Cumulative Avg u/s)',
            data: [%s],
            borderColor: '#2ea043',
            backgroundColor: 'rgba(46, 160, 67, 0.08)',
            borderWidth: 2.5,
            tension: 0.2,
            yAxisID: 'y',
            fill: false
          },
          {
            label: 'Speed Near Edge (50px from edge)',
            data: [%s],
            borderColor: '#f0883e',
            borderDash: [5, 5],
            borderWidth: 2,
            tension: 0.2,
            yAxisID: 'y',
            fill: false
          },
          {
            label: 'Precise Mode (units/s)',
            data: [%s],
            borderColor: '#38bdf8',
            borderWidth: 2,
            yAxisID: 'y',
            fill: false
          },
          {
            label: 'Acceleration (units/s²)',
            data: [%s],
            borderColor: '#a371f7',
            borderWidth: 1.5,
            borderDash: [2, 2],
            tension: 0.3,
            yAxisID: 'y1',
            fill: false
          }
        ]
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        interaction: {
          mode: 'index',
          intersect: false
        },
        plugins: {
          legend: {
            labels: { color: '#c9d1d9', font: { size: 13 } }
          },
          tooltip: {
            backgroundColor: '#161b22',
            titleColor: '#58a6ff',
            borderColor: '#30363d',
            borderWidth: 1
          }
        },
        scales: {
          x: {
            grid: { color: '#21262d' },
            ticks: { color: '#8b949e', maxTicksLimit: 12 },
            title: { display: true, text: 'Hold Duration', color: '#8b949e' }
          },
          y: {
            position: 'left',
            grid: { color: '#21262d' },
            ticks: { color: '#58a6ff' },
            title: { display: true, text: 'Velocity (units/s)', color: '#58a6ff' },
            min: 0,
            max: 2200
          },
          y1: {
            position: 'right',
            grid: { drawOnChartArea: false },
            ticks: { color: '#a371f7' },
            title: { display: true, text: 'Acceleration (units/s²)', color: '#a371f7' },
            min: 0
          }
        }
      }
    });
  </script>
</body>
</html>`,
		cfg.StartSpeed,
		cfg.MaxSpeed,
		effSpeedAt500ms,
		totalDistAtMax,
		cfg.TimeConstant,
		cfg.Power,
		cfg.InitialPressStep,
		`"`+strings.Join(labels, `","`)+`"`,
		strings.Join(normalData, ","),
		strings.Join(effSpeedData, ","),
		strings.Join(edgeFrictionData, ","),
		strings.Join(preciseData, ","),
		strings.Join(accelData, ","),
	)

	return os.WriteFile(path, []byte(htmlContent), 0644)
}
