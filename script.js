document.getElementById('download-form').addEventListener('submit', function() {
    document.getElementById('progress-container').style.display = 'block';
    document.getElementById('progress-text').textContent = '0%';
    document.getElementById('progress-bar').style.width = '0%';

    const interval = setInterval(() => {
        fetch('/progress')
            .then(response => response.json())
            .then(data => {
                const progress = Math.min(data.progress, 100);
                document.getElementById('progress-text').textContent = `${progress.toFixed(1)}%`;
                document.getElementById('progress-bar').style.width = `${progress}%`;
                if (progress >= 100) {
                    clearInterval(interval);
                    setTimeout(() => {
                        document.getElementById('progress-container').style.display = 'none';
                    }, 1000);
                }
            })
            .catch(() => {
                clearInterval(interval);
                document.getElementById('progress-container').style.display = 'none';
            });
    }, 500);
});