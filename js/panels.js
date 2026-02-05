class Panels {
    static initDraggable(element, handleSelector, storageKey) {
        const handle = element.querySelector(handleSelector) || element;
        let isDragging = false;
        let currentX;
        let currentY;
        let initialX;
        let initialY;
        let xOffset = 0;
        let yOffset = 0;

        // Load saved position
        const savedPos = localStorage.getItem(`panel-pos-${storageKey}`);
        if (savedPos) {
            const pos = JSON.parse(savedPos);
            xOffset = pos.x;
            yOffset = pos.y;
            element.style.transform = `translate3d(${xOffset}px, ${yOffset}px, 0)`;
        }

        handle.addEventListener('mousedown', dragStart);
        document.addEventListener('mousemove', drag);
        document.addEventListener('mouseup', dragEnd);

        function dragStart(e) {
            if (e.type === "mousedown") {
                initialX = e.clientX - xOffset;
                initialY = e.clientY - yOffset;
            }
            if (e.target === handle || handle.contains(e.target)) {
                isDragging = true;
            }
        }

        function drag(e) {
            if (isDragging) {
                e.preventDefault();
                currentX = e.clientX - initialX;
                currentY = e.clientY - initialY;
                xOffset = currentX;
                yOffset = currentY;
                setTranslate(currentX, currentY, element);
            }
        }

        function setTranslate(xPos, yPos, el) {
            el.style.transform = `translate3d(${xPos}px, ${yPos}px, 0)`;
        }

        function dragEnd(e) {
            if (isDragging) {
                initialX = currentX;
                initialY = currentY;
                isDragging = false;
                localStorage.setItem(`panel-pos-${storageKey}`, JSON.stringify({ x: xOffset, y: yOffset }));
            }
        }
    }

    static initResizable(element, handleSelector, storageKey, options = {}) {
        const handle = element.querySelector(handleSelector);
        if (!handle) return;

        let isResizing = false;
        let lastX;
        let lastY;
        
        const minWidth = options.minWidth || 100;
        const maxWidth = options.maxWidth || 1000;
        const minHeight = options.minHeight || 100;
        const maxHeight = options.maxHeight || 1000;

        // Load saved size
        const savedSize = localStorage.getItem(`panel-size-${storageKey}`);
        if (savedSize) {
            const size = JSON.parse(savedSize);
            if (size.width) element.style.width = `${size.width}px`;
            if (size.height) element.style.height = `${size.height}px`;
        }

        handle.addEventListener('mousedown', (e) => {
            isResizing = true;
            lastX = e.clientX;
            lastY = e.clientY;
            e.preventDefault();
        });

        document.addEventListener('mousemove', (e) => {
            if (!isResizing) return;

            const deltaX = e.clientX - lastX;
            const deltaY = e.clientY - lastY;
            
            if (options.axis !== 'y') {
                const newWidth = Math.min(maxWidth, Math.max(minWidth, element.offsetWidth + deltaX));
                element.style.width = `${newWidth}px`;
            }
            
            if (options.axis !== 'x') {
                const newHeight = Math.min(maxHeight, Math.max(minHeight, element.offsetHeight + deltaY));
                element.style.height = `${newHeight}px`;
            }

            lastX = e.clientX;
            lastY = e.clientY;
        });

        document.addEventListener('mouseup', () => {
            if (isResizing) {
                isResizing = false;
                localStorage.setItem(`panel-size-${storageKey}`, JSON.stringify({
                    width: element.offsetWidth,
                    height: element.offsetHeight
                }));
            }
        });
    }
}

window.Panels = Panels;
