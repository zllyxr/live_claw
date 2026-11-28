(function () {
  "use strict";

  function button(label, callback) {
    const el = document.createElement("button");
    el.type = "button";
    el.textContent = label;
    el.className = "layui-btn layui-btn-primary layui-btn-sm";
    el.addEventListener("click", callback);
    return el;
  }

  // The preview canvas is also the exported canvas, including for drag and zoom.
  function cropImage(parent, image, spec, confirm, reportError) {
    const dialog = document.createElement("dialog");
    dialog.className = "profile-crop-dialog";
    dialog.setAttribute("aria-label", "裁剪" + spec.label);
    const heading = document.createElement("h3");
    heading.textContent = "裁剪" + spec.label;
    const canvas = document.createElement("canvas");
    canvas.width = spec.width;
    canvas.height = spec.height;
    canvas.tabIndex = 0;
    canvas.setAttribute("aria-label", "拖动图片或使用方向键调整位置");
    const zoomLabel = document.createElement("label");
    zoomLabel.textContent = "缩放";
    const slider = document.createElement("input");
    slider.type = "range";
    slider.min = "1";
    slider.max = "4";
    slider.step = "0.01";
    slider.value = "1";
    zoomLabel.append(slider);
    const hint = document.createElement("p");
    hint.textContent = "拖动图片调整位置，也可以使用方向键。";
    const ctx = canvas.getContext("2d");
    if (!ctx) throw new Error("此浏览器无法裁剪图片");
    let centerX = image.naturalWidth / 2;
    let centerY = image.naturalHeight / 2;
    let cropWidth, cropHeight;
    function draw() {
      const scale = Math.max(canvas.width / image.naturalWidth, canvas.height / image.naturalHeight) * Number(slider.value);
      cropWidth = Math.min(image.naturalWidth, canvas.width / scale);
      cropHeight = Math.min(image.naturalHeight, canvas.height / scale);
      const x = Math.max(0, Math.min(image.naturalWidth - cropWidth, centerX - cropWidth / 2));
      const y = Math.max(0, Math.min(image.naturalHeight - cropHeight, centerY - cropHeight / 2));
      centerX = x + cropWidth / 2;
      centerY = y + cropHeight / 2;
      ctx.fillStyle = "#fff";
      ctx.fillRect(0, 0, canvas.width, canvas.height);
      ctx.drawImage(image, x, y, cropWidth, cropHeight, 0, 0, canvas.width, canvas.height);
    }
    let pointer;
    canvas.onpointerdown = (e) => {
      if (!e.isPrimary || e.button !== 0) return;
      canvas.setPointerCapture(e.pointerId);
      pointer = { id: e.pointerId, x: e.clientX, y: e.clientY };
    };
    canvas.onpointermove = (e) => {
      if (!pointer || pointer.id !== e.pointerId) return;
      const box = canvas.getBoundingClientRect();
      centerX -= (e.clientX - pointer.x) * cropWidth / box.width;
      centerY -= (e.clientY - pointer.y) * cropHeight / box.height;
      pointer.x = e.clientX;
      pointer.y = e.clientY;
      draw();
    };
    canvas.onlostpointercapture = () => { pointer = null; };
    canvas.onkeydown = (e) => {
      const move = { ArrowLeft: [1, 0], ArrowRight: [-1, 0], ArrowUp: [0, 1], ArrowDown: [0, -1] }[e.key];
      if (!move) return;
      e.preventDefault();
      centerX += move[0] * cropWidth / 30;
      centerY += move[1] * cropHeight / 30;
      draw();
    };
    slider.oninput = draw;
    const actions = document.createElement("div");
    actions.className = "profile-image-actions";
    actions.append(button("取消", () => dialog.close()), button("使用裁剪结果", () => {
      try {
        confirm(canvas.toDataURL("image/jpeg", 0.9));
        dialog.close();
      } catch (_) {
        dialog.close();
        reportError("无法读取此图片，请重新上传后裁剪");
      }
    }));
    dialog.append(heading, canvas, hint, zoomLabel, actions);
    parent.append(dialog);
    dialog.addEventListener("close", () => dialog.remove(), { once: true });
    draw();
    dialog.showModal();
    canvas.focus();
  }

  function mount(form, row, changes) {
    const group = document.createElement("div");
    group.className = "profile-image-fields";
    form.append(group);
    for (const spec of [
      { key: "avatar", label: "头像", width: 512, height: 512 },
      { key: "background", label: "背景", width: 1200, height: 675 }
    ]) {
      const field = document.createElement("section");
      field.className = "profile-image-field profile-image-" + spec.key;
      const title = document.createElement("strong");
      title.textContent = spec.label;
      const original = row[spec.key + "_url"] || "";
      const preview = document.createElement("img");
      preview.alt = "当前" + spec.label;
      const note = document.createElement("small");
      note.setAttribute("role", "status");
      const file = document.createElement("input");
      file.type = "file";
      file.accept = "image/*";
      file.hidden = true;
      let revision = 0;
      let loading = false;
      function refresh(url, message) {
        if (url) preview.src = url;
        else preview.removeAttribute("src");
        preview.hidden = !url;
        edit.disabled = !url || loading;
        upload.disabled = loading;
        undo.hidden = !changes[spec.key];
        note.textContent = message || (url ? "" : "尚未设置");
      }
      async function open(source, localFile) {
        const current = ++revision;
        loading = true;
        field.setAttribute("aria-busy", "true");
        refresh(preview.getAttribute("src"), "正在读取图片…");
        const img = new Image();
        img.crossOrigin = "anonymous";
        try {
          img.src = source;
          await img.decode();
          if (current !== revision || !form.isConnected) return;
          cropImage(form, img, spec, (dataURL) => {
            changes[spec.key] = { previous_id: row[spec.key + "_asset_id"], data: dataURL.split(",")[1] };
            refresh(dataURL, "已裁剪，保存后生效");
          }, (message) => { note.textContent = message; });
          note.textContent = "";
        } catch (_) {
          if (current === revision) note.textContent = "无法读取图片，请选择其他图片";
        } finally {
          if (localFile) URL.revokeObjectURL(source);
          if (current === revision) {
            loading = false;
            field.removeAttribute("aria-busy");
            upload.disabled = false;
            edit.disabled = !preview.getAttribute("src");
          }
        }
      }
      const upload = button("上传" + spec.label, () => file.click());
      const edit = button("裁剪", () => open(preview.getAttribute("src"), false));
      const undo = button("撤销修改", () => {
        ++revision;
        loading = false;
        field.removeAttribute("aria-busy");
        delete changes[spec.key];
        refresh(original);
      });
      file.onchange = () => {
        const selected = file.files[0];
        file.value = "";
        if (selected) open(URL.createObjectURL(selected), true);
      };
      const actions = document.createElement("div");
      actions.className = "profile-image-actions";
      actions.append(upload, edit, undo);
      field.append(title, preview, actions, file, note);
      group.append(field);
      refresh(original);
    }
  }

  window.ClawProfileImages = { mount };
})();
