from flask import Flask, request, render_template, send_from_directory, jsonify
import requests
from bs4 import BeautifulSoup
import os
import re
import shutil
import zipfile
import time
import random

app = Flask(__name__)

# Thư mục lưu file tải xuống
DOWNLOAD_DIR = "downloads"
if not os.path.exists(DOWNLOAD_DIR):
    os.makedirs(DOWNLOAD_DIR)

# Danh sách server mirror (giả định)
MIRROR_SERVERS = [
    "https://main-server.com",
    "https://mirror1-server.com",
    "https://mirror2-server.com"
]

def clean_filename(title):
    """Làm sạch tiêu đề để tạo tên file/thư mục hợp lệ"""
    return re.sub(r'[^\w\s-]', '', title).strip().replace(' ', '_')

def get_chapter_title(soup, url):
    """Lấy tiêu đề chương từ HTML hoặc URL"""
    title_tag = soup.find('h1') or soup.find('h2')
    if title_tag:
        return title_tag.get_text().strip()
    return url.split('/')[-1] or "unknown_chapter"

def download_chapter_images(url, chapter_dir, progress_callback=None):
    """Tải hình ảnh từ URL chương truyện với xử lý lỗi và thời gian chờ"""
    attempts = 0
    max_attempts = 3
    delay = 1  # Thời gian chờ ban đầu (giây)

    for server in MIRROR_SERVERS:
        current_url = url.replace(MIRROR_SERVERS[0], server) if MIRROR_SERVERS[0] in url else url
        while attempts < max_attempts:
            try:
                headers = {'User-Agent': 'Mozilla/5.0'}
                response = requests.get(current_url, headers=headers, timeout=10)
                if response.status_code == 429:  # Too Many Requests
                    delay *= 2  # Tăng thời gian chờ gấp đôi
                    time.sleep(delay + random.uniform(0, 0.5))
                    attempts += 1
                    continue
                response.raise_for_status()
                break
            except requests.RequestException as e:
                attempts += 1
                if attempts == max_attempts:
                    if server == MIRROR_SERVERS[-1]:
                        return None, f"Lỗi tải từ tất cả server: {str(e)}"
                    break
                time.sleep(delay)
        else:
            continue
        break
    else:
        return None, "Không thể kết nối đến bất kỳ server nào."

    soup = BeautifulSoup(response.text, 'html.parser')
    image_tags = soup.find_all('img', class_='chapter-image') or \
                 soup.find('div', class_='chapter-content').find_all('img') if soup.find('div', class_='chapter-content') else []

    if not image_tags:
        return None, "Không tìm thấy hình ảnh nào."

    total_images = len(image_tags)
    downloaded = 0

    for idx, img in enumerate(image_tags):
        img_url = img.get('src')
        if not img_url:
            continue
        if not img_url.startswith('http'):
            img_url = current_url.rsplit('/', 1)[0] + '/' + img_url.lstrip('/')
        
        attempts = 0
        while attempts < max_attempts:
            try:
                img_response = requests.get(img_url, headers=headers, timeout=10)
                if img_response.status_code == 429:
                    delay *= 2
                    time.sleep(delay + random.uniform(0, 0.5))
                    attempts += 1
                    continue
                img_response.raise_for_status()
                img_name = f"image_{idx + 1}.jpg"
                img_path = os.path.join(chapter_dir, img_name)
                with open(img_path, 'wb') as f:
                    f.write(img_response.content)
                downloaded += 1
                if progress_callback:
                    progress_callback(downloaded, total_images)
                break
            except requests.RequestException:
                attempts += 1
                if attempts == max_attempts:
                    break
                time.sleep(delay)

    return downloaded, None

@app.route('/')
def index():
    return render_template('index.html')

@app.route('/download', methods=['POST'])
def download():
    url = request.form.get('url')
    if not url:
        return render_template('index.html', error="Vui lòng nhập URL hợp lệ.")

    # Tạo thư mục cho chương
    response = requests.get(url)
    soup = BeautifulSoup(response.text, 'html.parser')
    chapter_title = get_chapter_title(soup, url)
    chapter_dir = os.path.join(DOWNLOAD_DIR, clean_filename(chapter_title))
    if not os.path.exists(chapter_dir):
        os.makedirs(chapter_dir)

    # Tải hình ảnh
    progress = {'downloaded': 0, 'total': 0}
    def update_progress(downloaded, total):
        progress['downloaded'] = downloaded
        progress['total'] = total

    num_images, error = download_chapter_images(url, chapter_dir, update_progress)
    if error:
        shutil.rmtree(chapter_dir, ignore_errors=True)
        return render_template('index.html', error=f"Lỗi: {error}")

    # Tạo file ZIP
    zip_filename = f"{clean_filename(chapter_title)}.zip"
    zip_path = os.path.join(DOWNLOAD_DIR, zip_filename)
    with zipfile.ZipFile(zip_path, 'w', zipfile.ZIP_DEFLATED) as zipf:
        for root, _, files in os.walk(chapter_dir):
            for file in files:
                zipf.write(os.path.join(root, file), os.path.join(chapter_title, file))

    return render_template('index.html', success=True, zip_filename=zip_filename, 
                         num_images=num_images, chapter_title=chapter_title)

@app.route('/progress')
def get_progress():
    return jsonify({'progress': progress['downloaded'] / progress['total'] * 100 if progress['total'] else 0})

@app.route('/download_zip/<zip_filename>')
def download_zip(zip_filename):
    zip_path = os.path.join(DOWNLOAD_DIR, zip_filename)
    if os.path.exists(zip_path):
        return send_from_directory(DOWNLOAD_DIR, zip_filename, as_attachment=True)
    return "File không tồn tại.", 404

if __name__ == '__main__':
    app.run(debug=True)