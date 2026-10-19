import { Service} from '@angular/core';
import { Observable } from 'rxjs';

import { ApiService } from './api.service';

@Service()
export class ImageUploadService extends ApiService {
  protected endpoint = 'crm/tinymce/upload-image';

  // REF: https://youtu.be/YkvqLNcJz3Y
  uploadImage(blobInfo: any): Observable<any> {
    const formData = new FormData();
    formData.append('image', blobInfo.blob(), blobInfo.filename());
    return this.post(this.endpoint, formData);
  }
}
