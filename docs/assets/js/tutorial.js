// Ultra-Fast Networking Tutorial JavaScript
// Provides interactive features for the tutorial website

class TutorialEngine {
    constructor() {
        this.progress = this.loadProgress();
        this.currentChapter = 1;
        this.init();
    }

    init() {
        this.setupEventListeners();
        this.setupProgressTracking();
        this.setupCodeEditors();
        this.setupInteractiveElements();
        this.loadUserPreferences();
    }

    setupEventListeners() {
        document.addEventListener('DOMContentLoaded', () => {
            this.initializeChapter();
        });

        // Smooth scrolling for navigation links
        document.querySelectorAll('a[href^="#"]').forEach(anchor => {
            anchor.addEventListener('click', (e) => {
                e.preventDefault();
                const target = document.querySelector(anchor.getAttribute('href'));
                if (target) {
                    target.scrollIntoView({
                        behavior: 'smooth',
                        block: 'start'
                    });
                }
            });
        });

        // Track external links
        document.querySelectorAll('a[href^="http"]').forEach(link => {
            link.addEventListener('click', (e) => {
                this.trackEvent('external_link', link.href);
            });
        });
    }

    setupProgressTracking() {
        // Intersection Observer for section tracking
        if ('IntersectionObserver' in window) {
            const observer = new IntersectionObserver((entries) => {
                entries.forEach(entry => {
                    if (entry.isIntersecting) {
                        this.updateSectionProgress(entry.target);
                    }
                });
            }, { threshold: 0.5 });

            document.querySelectorAll('.content-section').forEach(section => {
                observer.observe(section);
            });
        }

        // Chapter completion tracking
        this.setupChapterCompletion();
    }

    setupChapterCompletion() {
        // Track when users complete chapters
        const checkboxes = document.querySelectorAll('.knowledge-check input[type="checkbox"]');
        checkboxes.forEach(checkbox => {
            checkbox.addEventListener('change', () => {
                this.updateKnowledgeCheck();
            });
        });
    }

    setupCodeEditors() {
        // Initialize syntax highlighting
        if (typeof hljs !== 'undefined') {
            document.querySelectorAll('pre code').forEach(block => {
                hljs.highlightBlock(block);
            });
        }

        // Setup code execution simulation
        this.setupCodeExecution();
    }

    setupCodeExecution() {
        // Simulate running tests and benchmarks
        window.runTest = (testName) => {
            this.simulateTestExecution(testName);
        };

        window.runBenchmark = () => {
            this.simulateBenchmark();
        };

        window.copyCode = (button) => {
            this.copyCodeToClipboard(button);
        };
    }

    simulateTestExecution(testName) {
        const output = document.querySelector('.terminal-content pre code');
        if (!output) return;

        output.innerHTML = `<span class="running">Running ${testName}...</span>`;
        
        setTimeout(() => {
            const results = this.generateTestResults(testName);
            output.innerHTML = results;
            this.trackEvent('test_run', testName);
        }, Math.random() * 1000 + 500); // Random delay 0.5-1.5s
    }

    generateTestResults(testName) {
        const testResults = {
            'TestLinuxSocketCreation': {
                success: true,
                duration: '0.001s',
                output: 'Socket created successfully with fd=3'
            },
            'TestLinuxSocketBinding': {
                success: true,
                duration: '0.002s', 
                output: 'Bound to 127.0.0.1:43251'
            },
            'TestLinuxSocketPerformance': {
                success: true,
                duration: '0.150s',
                output: 'Performance: 250000 packets/second, 4.00 μs/packet'
            }
        };

        const result = testResults[testName] || {
            success: true,
            duration: '0.001s',
            output: 'Test completed successfully'
        };

        return `$ go test -v -run ${testName}

=== RUN   ${testName}
    ${result.output}
--- PASS: ${testName} (${result.duration})
PASS

<span class="success">✅ Test passed!</span>`;
    }

    simulateBenchmark() {
        const output = document.querySelector('.terminal-content pre code');
        if (!output) return;

        output.innerHTML = `<span class="running">Running performance benchmark...</span>`;
        
        setTimeout(() => {
            output.innerHTML = `$ go test -bench=. -v

BenchmarkLinuxSocket-8     250000    4.2 μs/op    0 allocs/op
BenchmarkNetUDPConn-8      150000    6.7 μs/op    2 allocs/op
BenchmarkZeroCopy-8        400000    2.8 μs/op    0 allocs/op

<span class="success">✅ Raw socket is 67% faster than net package!</span>
<span class="success">✅ Zero-copy is 140% faster than standard approach!</span>`;
            
            this.trackEvent('benchmark_run');
        }, 2000);
    }

    copyCodeToClipboard(button) {
        const codeBlock = button.closest('.code-editor').querySelector('code');
        const text = codeBlock.textContent;
        
        if (navigator.clipboard) {
            navigator.clipboard.writeText(text).then(() => {
                this.showCopyFeedback(button);
                this.trackEvent('code_copy');
            }).catch(err => {
                console.error('Failed to copy code:', err);
                this.fallbackCopyToClipboard(text, button);
            });
        } else {
            this.fallbackCopyToClipboard(text, button);
        }
    }

    fallbackCopyToClipboard(text, button) {
        const textArea = document.createElement('textarea');
        textArea.value = text;
        document.body.appendChild(textArea);
        textArea.select();
        
        try {
            document.execCommand('copy');
            this.showCopyFeedback(button);
            this.trackEvent('code_copy');
        } catch (err) {
            console.error('Fallback copy failed:', err);
        }
        
        document.body.removeChild(textArea);
    }

    showCopyFeedback(button) {
        const originalText = button.textContent;
        button.textContent = 'Copied!';
        button.classList.add('copied');
        
        setTimeout(() => {
            button.textContent = originalText;
            button.classList.remove('copied');
        }, 2000);
    }

    setupInteractiveElements() {
        // Roadmap item interactions
        document.querySelectorAll('.roadmap-item').forEach(item => {
            item.addEventListener('click', () => {
                const chapter = item.dataset.chapter;
                if (chapter) {
                    window.location.href = `chapter${chapter}/`;
                    this.trackEvent('chapter_click', chapter);
                }
            });

            // Add hover effects
            item.addEventListener('mouseenter', () => {
                item.style.transform = 'translateX(12px)';
            });

            item.addEventListener('mouseleave', () => {
                item.style.transform = 'translateX(0)';
            });
        });

        // Performance chart interactions
        this.setupPerformanceCharts();
        
        // Interactive demos
        this.setupInteractiveDemos();
    }

    setupPerformanceCharts() {
        const latencyChart = document.getElementById('latencyChart');
        const throughputChart = document.getElementById('throughputChart');

        if (latencyChart) {
            this.createLatencyChart(latencyChart);
        }

        if (throughputChart) {
            this.createThroughputChart(throughputChart);
        }
    }

    createLatencyChart(canvas) {
        const ctx = canvas.getContext('2d');
        const data = {
            traditional: [5000, 8000, 12000, 15000, 20000], // microseconds
            ultrafast: [50, 80, 120, 150, 200]
        };

        this.drawBarChart(ctx, canvas, data, 'Latency Comparison (μs)', '#ff6b6b', '#4ecdc4');
    }

    createThroughputChart(canvas) {
        const ctx = canvas.getContext('2d');
        const data = {
            traditional: [50000, 80000, 120000, 150000, 200000], // requests/sec
            ultrafast: [500000, 800000, 1200000, 1500000, 2000000]
        };

        this.drawBarChart(ctx, canvas, data, 'Throughput Comparison (req/s)', '#45b7d1', '#96c93f');
    }

    drawBarChart(ctx, canvas, data, title, color1, color2) {
        const width = canvas.width;
        const height = canvas.height;
        const margin = 40;

        // Clear canvas
        ctx.fillStyle = '#f8f9fa';
        ctx.fillRect(0, 0, width, height);

        // Draw title
        ctx.fillStyle = '#2d3748';
        ctx.font = 'bold 16px sans-serif';
        ctx.textAlign = 'center';
        ctx.fillText(title, width / 2, 25);

        // Draw bars
        const barWidth = (width - 2 * margin) / 10;
        const maxValue = Math.max(...data.traditional, ...data.ultrafast);

        data.traditional.forEach((value, i) => {
            const barHeight = (value / maxValue) * (height - 2 * margin - 30);
            const x = margin + i * 2 * barWidth;
            const y = height - margin - barHeight;

            ctx.fillStyle = color1;
            ctx.fillRect(x, y, barWidth * 0.8, barHeight);
        });

        data.ultrafast.forEach((value, i) => {
            const barHeight = (value / maxValue) * (height - 2 * margin - 30);
            const x = margin + i * 2 * barWidth + barWidth;
            const y = height - margin - barHeight;

            ctx.fillStyle = color2;
            ctx.fillRect(x, y, barWidth * 0.8, barHeight);
        });

        // Legend
        ctx.fillStyle = color1;
        ctx.fillRect(margin, height - 20, 15, 10);
        ctx.fillStyle = '#2d3748';
        ctx.font = '12px sans-serif';
        ctx.textAlign = 'left';
        ctx.fillText('Traditional', margin + 20, height - 12);

        ctx.fillStyle = color2;
        ctx.fillRect(margin + 120, height - 20, 15, 10);
        ctx.fillText('Ultra-Fast', margin + 140, height - 12);
    }

    setupInteractiveDemos() {
        // Add interactive elements for demonstrations
        const demoButtons = document.querySelectorAll('.demo-button');
        demoButtons.forEach(button => {
            button.addEventListener('click', () => {
                this.runDemo(button.dataset.demo);
            });
        });
    }

    runDemo(demoType) {
        switch (demoType) {
            case 'socket-creation':
                this.demoSocketCreation();
                break;
            case 'performance-test':
                this.demoPerformanceTest();
                break;
            default:
                console.log('Demo not implemented:', demoType);
        }
    }

    demoSocketCreation() {
        // Simulate socket creation process
        const steps = [
            'Creating socket with syscall.Socket(AF_INET, SOCK_DGRAM, 0)',
            'Setting socket options for performance',
            'Binding to address 127.0.0.1:0',
            'Socket ready! File descriptor: 3'
        ];

        let currentStep = 0;
        const interval = setInterval(() => {
            console.log(steps[currentStep]);
            currentStep++;
            
            if (currentStep >= steps.length) {
                clearInterval(interval);
                console.log('✅ Socket creation demo complete!');
            }
        }, 1000);
    }

    updateSectionProgress(section) {
        const stepNumber = parseInt(section.dataset.step);
        if (stepNumber) {
            this.progress.currentStep = Math.max(this.progress.currentStep, stepNumber);
            this.progress.lastUpdated = Date.now();
            this.saveProgress();
            this.updateProgressUI();
        }
    }

    updateKnowledgeCheck() {
        const checkboxes = document.querySelectorAll('.knowledge-check input[type="checkbox"]');
        const checkedBoxes = document.querySelectorAll('.knowledge-check input[type="checkbox"]:checked');
        
        const completion = (checkedBoxes.length / checkboxes.length) * 100;
        
        if (completion === 100) {
            this.markChapterComplete();
        }
    }

    updateProgressUI() {
        const progressBar = document.querySelector('.progress-fill');
        const progressText = document.querySelector('.progress-text');
        
        if (progressBar && progressText) {
            const totalSteps = document.querySelectorAll('.content-section').length;
            const percentage = (this.progress.currentStep / totalSteps) * 100;
            
            progressBar.style.width = percentage + '%';
            progressText.textContent = Math.round(percentage) + '% Complete';
        }
    }

    markChapterComplete() {
        this.progress.chaptersCompleted.push(this.currentChapter);
        this.progress.completedAt = Date.now();
        this.saveProgress();
        
        this.showCompletionMessage();
        this.trackEvent('chapter_complete', this.currentChapter);
    }

    showCompletionMessage() {
        const message = document.createElement('div');
        message.className = 'completion-message';
        message.innerHTML = `
            <div class="completion-content">
                <h3>🎉 Chapter Complete!</h3>
                <p>Congratulations! You've mastered the concepts in this chapter.</p>
                <p>Ready to move on to the next challenge?</p>
                <button onclick="this.parentElement.parentElement.remove()">Continue</button>
            </div>
        `;
        
        document.body.appendChild(message);
        
        setTimeout(() => {
            message.remove();
        }, 10000);
    }

    loadProgress() {
        const saved = localStorage.getItem('tutorial-progress');
        if (saved) {
            return JSON.parse(saved);
        }
        
        return {
            currentStep: 0,
            chaptersCompleted: [],
            lastUpdated: Date.now(),
            completedAt: null
        };
    }

    saveProgress() {
        localStorage.setItem('tutorial-progress', JSON.stringify(this.progress));
    }

    loadUserPreferences() {
        const prefs = localStorage.getItem('tutorial-preferences');
        if (prefs) {
            const preferences = JSON.parse(prefs);
            this.applyPreferences(preferences);
        }
    }

    applyPreferences(preferences) {
        // Apply theme
        if (preferences.theme) {
            document.body.classList.add(`theme-${preferences.theme}`);
        }
        
        // Apply font size
        if (preferences.fontSize) {
            document.documentElement.style.fontSize = preferences.fontSize + 'px';
        }
    }

    initializeChapter() {
        // Chapter-specific initialization
        const chapterMeta = document.querySelector('.chapter-meta');
        if (chapterMeta) {
            const chapterNumber = chapterMeta.querySelector('.chapter-number');
            if (chapterNumber) {
                this.currentChapter = parseInt(chapterNumber.textContent.match(/\d+/)[0]);
            }
        }

        this.updateProgressUI();
        this.setupChapterNavigation();
    }

    setupChapterNavigation() {
        // Keyboard shortcuts for navigation
        document.addEventListener('keydown', (e) => {
            if (e.ctrlKey || e.metaKey) {
                switch (e.key) {
                    case 'ArrowLeft':
                        this.navigateToPreviousChapter();
                        break;
                    case 'ArrowRight':
                        this.navigateToNextChapter();
                        break;
                }
            }
        });
    }

    navigateToPreviousChapter() {
        const prevButton = document.querySelector('.nav-btn.nav-prev');
        if (prevButton) {
            window.location.href = prevButton.href;
        }
    }

    navigateToNextChapter() {
        const nextButton = document.querySelector('.nav-btn.nav-next');
        if (nextButton) {
            window.location.href = nextButton.href;
        }
    }

    trackEvent(eventName, eventData = null) {
        // Simple event tracking for analytics
        console.log('Event:', eventName, eventData);
        
        // You could integrate with Google Analytics, Mixpanel, etc. here
        if (typeof gtag !== 'undefined') {
            gtag('event', eventName, {
                'custom_parameter': eventData
            });
        }
    }
}

// Performance monitoring
class PerformanceMonitor {
    constructor() {
        this.metrics = {};
        this.startTime = performance.now();
    }

    mark(name) {
        this.metrics[name] = performance.now() - this.startTime;
    }

    measure(startMark, endMark) {
        return this.metrics[endMark] - this.metrics[startMark];
    }

    getMetrics() {
        return this.metrics;
    }
}

// Initialize tutorial when DOM is ready
document.addEventListener('DOMContentLoaded', () => {
    window.tutorialEngine = new TutorialEngine();
    window.performanceMonitor = new PerformanceMonitor();
    
    // Mark initialization complete
    window.performanceMonitor.mark('tutorial_initialized');
});

// Export for use in other scripts
if (typeof module !== 'undefined' && module.exports) {
    module.exports = { TutorialEngine, PerformanceMonitor };
}