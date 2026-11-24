import os
import tempfile
import unittest

import app


class MediaTest(unittest.TestCase):
    def test_project_isolation_and_active_content(self):
        with tempfile.TemporaryDirectory() as root:
            old_root = app.DATA_ROOT
            app.DATA_ROOT = root
            try:
                for project, content in (("one", b"first"), ("two", b"second")):
                    directory = os.path.join(root, project, "docs")
                    os.makedirs(directory)
                    with open(os.path.join(directory, "same.html"), "wb") as file:
                        file.write(content)
                client = app.app.test_client()
                response = client.get("/api/hpahub/media/file/one/docs/same.html")
                self.assertEqual(response.data, b"first")
                self.assertEqual(response.headers["Content-Disposition"], "attachment")
                self.assertEqual(client.get("/api/hpahub/media/file/missing/docs/same.html").status_code, 404)
                os.symlink(os.path.join(root, "two", "docs", "same.html"), os.path.join(root, "one", "docs", "link.html"))
                self.assertEqual(client.get("/api/hpahub/media/file/one/docs/link.html").status_code, 404)
                self.assertEqual(client.get("/api/hpahub/media/file/one/docs/.meta.json").status_code, 404)
            finally:
                app.DATA_ROOT = old_root


if __name__ == "__main__":
    unittest.main()
